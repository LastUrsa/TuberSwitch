package obs

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"TuberSwitch/internal/config"

	"github.com/gorilla/websocket"
)

type Client struct {
	logger *log.Logger
	mu     sync.Mutex
	conn   *websocket.Conn
}

type Scene struct {
	Name string
}

type Source struct {
	Name        string
	SceneItemID int
}

func New(logger *log.Logger) *Client {
	return &Client{logger: logger}
}

func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) Connect(cfg config.OBSConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return nil
	}

	conn, err := c.dial(cfg)
	if err != nil {
		return fmt.Errorf("OBS connection failed: %w", err)
	}

	var hello obsMessage
	if err := conn.ReadJSON(&hello); err != nil {
		_ = conn.Close()
		return fmt.Errorf("OBS hello failed: %w", err)
	}
	if hello.Op != 0 {
		_ = conn.Close()
		return fmt.Errorf("OBS expected hello, got op %d", hello.Op)
	}

	identify := map[string]interface{}{"rpcVersion": 1}
	var helloData helloData
	_ = mapToStruct(hello.D, &helloData)
	if helloData.Authentication != nil {
		auth, err := buildAuth(cfg.Password, helloData.Authentication.Salt, helloData.Authentication.Challenge)
		if err != nil {
			_ = conn.Close()
			return err
		}
		identify["authentication"] = auth
	}
	if err := conn.WriteJSON(obsMessage{Op: 1, D: identify}); err != nil {
		_ = conn.Close()
		return fmt.Errorf("OBS identify failed: %w", err)
	}

	var identified obsMessage
	if err := conn.ReadJSON(&identified); err != nil {
		_ = conn.Close()
		return fmt.Errorf("OBS identify response failed: %w", err)
	}
	if identified.Op != 2 {
		_ = conn.Close()
		return fmt.Errorf("OBS authentication failed or unexpected op %d", identified.Op)
	}

	c.conn = conn
	c.logger.Printf("OBS connected")
	return nil
}

func (c *Client) dial(cfg config.OBSConfig) (*websocket.Conn, error) {
	if err := validateHost(cfg); err != nil {
		return nil, err
	}
	hosts := []string{cfg.Host}
	if cfg.Host == "localhost" {
		hosts = append(hosts, "127.0.0.1")
	}
	var lastErr error
	scheme := transportScheme(cfg)
	for _, host := range hosts {
		u := url.URL{Scheme: scheme, Host: host + ":" + strconv.Itoa(cfg.Port), Path: "/"}
		c.logger.Printf("OBS connection attempt: %s", u.String())
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *Client) GetScenes() ([]Scene, error) {
	var response struct {
		Scenes []struct {
			SceneName string `json:"sceneName"`
		} `json:"scenes"`
	}
	if err := c.request("GetSceneList", nil, &response); err != nil {
		return nil, err
	}
	scenes := make([]Scene, 0, len(response.Scenes))
	for _, scene := range response.Scenes {
		scenes = append(scenes, Scene{Name: scene.SceneName})
	}
	return scenes, nil
}

func (c *Client) GetSources(sceneName string) ([]Source, error) {
	var response struct {
		SceneItems []struct {
			SourceName  string `json:"sourceName"`
			SceneItemID int    `json:"sceneItemId"`
		} `json:"sceneItems"`
	}
	if err := c.request("GetSceneItemList", map[string]interface{}{"sceneName": sceneName}, &response); err != nil {
		return nil, err
	}
	sources := make([]Source, 0, len(response.SceneItems))
	for _, item := range response.SceneItems {
		sources = append(sources, Source{Name: item.SourceName, SceneItemID: item.SceneItemID})
	}
	return sources, nil
}

func (c *Client) FindSceneItemID(sceneName string, sourceName string) (int, error) {
	sources, err := c.GetSources(sceneName)
	if err != nil {
		return 0, err
	}
	for _, source := range sources {
		if source.Name == sourceName {
			return source.SceneItemID, nil
		}
	}
	return 0, fmt.Errorf("OBS source %q was not found in scene %q", sourceName, sceneName)
}

func (c *Client) SetSourceVisibility(sceneName string, sourceName string, sceneItemID int, enabled bool) error {
	if sceneName == "" {
		return fmt.Errorf("OBS scene name is required")
	}
	if sceneItemID == 0 {
		id, err := c.FindSceneItemID(sceneName, sourceName)
		if err != nil {
			return err
		}
		sceneItemID = id
	}
	data := map[string]interface{}{
		"sceneName":        sceneName,
		"sceneItemId":      sceneItemID,
		"sceneItemEnabled": enabled,
	}
	if err := c.request("SetSceneItemEnabled", data, nil); err != nil {
		return fmt.Errorf("OBS failed setting %q visibility to %v: %w", sourceName, enabled, err)
	}
	return nil
}

func (c *Client) request(requestType string, requestData map[string]interface{}, out interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("OBS is disconnected")
	}
	id := fmt.Sprintf("%s-%d", requestType, time.Now().UnixNano())
	if requestData == nil {
		requestData = map[string]interface{}{}
	}
	msg := obsMessage{
		Op: 6,
		D: map[string]interface{}{
			"requestType": requestType,
			"requestId":   id,
			"requestData": requestData,
		},
	}
	if err := c.conn.WriteJSON(msg); err != nil {
		c.closeLocked()
		return err
	}
	for {
		var response obsMessage
		if err := c.conn.ReadJSON(&response); err != nil {
			c.closeLocked()
			return err
		}
		if response.Op != 7 {
			continue
		}
		var data requestResponse
		if err := mapToStruct(response.D, &data); err != nil {
			return err
		}
		if data.RequestID != id {
			continue
		}
		if !data.RequestStatus.Result {
			return fmt.Errorf("%v: %s", data.RequestStatus.Code, data.RequestStatus.Comment)
		}
		if out != nil {
			return mapToStruct(data.ResponseData, out)
		}
		return nil
	}
}

func (c *Client) closeLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

type obsMessage struct {
	Op int         `json:"op"`
	D  interface{} `json:"d"`
}

type helloData struct {
	Authentication *struct {
		Challenge string `json:"challenge"`
		Salt      string `json:"salt"`
	} `json:"authentication"`
}

type requestResponse struct {
	RequestID     string `json:"requestId"`
	RequestStatus struct {
		Result  bool        `json:"result"`
		Code    interface{} `json:"code"`
		Comment string      `json:"comment"`
	} `json:"requestStatus"`
	ResponseData interface{} `json:"responseData"`
}

func buildAuth(password string, salt string, challenge string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("OBS password is required")
	}
	secretHash := sha256.Sum256([]byte(password + salt))
	secret := base64.StdEncoding.EncodeToString(secretHash[:])
	authHash := sha256.Sum256([]byte(secret + challenge))
	return base64.StdEncoding.EncodeToString(authHash[:]), nil
}

func mapToStruct(input interface{}, out interface{}) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func validateHost(cfg config.OBSConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("OBS host is required")
	}
	if cfg.AllowRemote {
		return nil
	}
	if !isLoopbackHost(cfg.Host) {
		return fmt.Errorf("remote OBS hosts are disabled; use a loopback host or explicitly allow remote OBS connections")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	switch host {
	case "localhost":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func transportScheme(cfg config.OBSConfig) string {
	if cfg.AllowRemote && !isLoopbackHost(cfg.Host) {
		return "wss"
	}
	return "ws"
}
