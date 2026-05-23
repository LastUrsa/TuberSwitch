package obs

import (
	"crypto/sha256"
	"encoding/base64"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"TuberSwitch/internal/config"

	"github.com/gorilla/websocket"
)

func TestBuildAuth(t *testing.T) {
	got, err := buildAuth("password", "salt", "challenge")
	if err != nil {
		t.Fatalf("buildAuth: %v", err)
	}
	secretHash := sha256.Sum256([]byte("password" + "salt"))
	secret := base64.StdEncoding.EncodeToString(secretHash[:])
	authHash := sha256.Sum256([]byte(secret + "challenge"))
	want := base64.StdEncoding.EncodeToString(authHash[:])
	if got != want {
		t.Fatalf("auth = %q, want %q", got, want)
	}
}

func TestBuildAuthRequiresPassword(t *testing.T) {
	if _, err := buildAuth("", "salt", "challenge"); err == nil {
		t.Fatalf("expected password error")
	}
}

func TestMapToStruct(t *testing.T) {
	var out struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	}
	err := mapToStruct(map[string]interface{}{"name": "Main", "id": 7}, &out)
	if err != nil {
		t.Fatalf("mapToStruct: %v", err)
	}
	if out.Name != "Main" || out.ID != 7 {
		t.Fatalf("out = %#v", out)
	}
}

func TestClientWebSocketHandshakeAndRequests(t *testing.T) {
	var setVisibilityPayload map[string]interface{}
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		if err := conn.WriteJSON(obsMessage{Op: 0, D: map[string]interface{}{"obsWebSocketVersion": "5.0.0", "rpcVersion": 1}}); err != nil {
			t.Fatalf("write hello: %v", err)
		}
		var identify obsMessage
		if err := conn.ReadJSON(&identify); err != nil {
			t.Fatalf("read identify: %v", err)
		}
		if identify.Op != 1 {
			t.Fatalf("identify op = %d", identify.Op)
		}
		if err := conn.WriteJSON(obsMessage{Op: 2, D: map[string]interface{}{"negotiatedRpcVersion": 1}}); err != nil {
			t.Fatalf("write identified: %v", err)
		}

		for {
			var request obsMessage
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			data := request.D.(map[string]interface{})
			requestID := data["requestId"].(string)
			requestType := data["requestType"].(string)
			responseData := map[string]interface{}{}
			switch requestType {
			case "GetSceneList":
				responseData["scenes"] = []map[string]interface{}{{"sceneName": "Main"}}
			case "GetSceneItemList":
				responseData["sceneItems"] = []map[string]interface{}{
					{"sourceName": "VTuber", "sceneItemId": 10},
					{"sourceName": "PNG", "sceneItemId": 11},
				}
			case "SetSceneItemEnabled":
				setVisibilityPayload = data["requestData"].(map[string]interface{})
			default:
				t.Fatalf("unexpected request type %q", requestType)
			}
			err := conn.WriteJSON(obsMessage{Op: 7, D: map[string]interface{}{
				"requestId": requestID,
				"requestStatus": map[string]interface{}{
					"result": true,
					"code":   100,
				},
				"responseData": responseData,
			}})
			if err != nil {
				t.Fatalf("write response: %v", err)
			}
		}
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	port, err := strconv.Atoi(serverURL.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	client := New(log.Default())
	if err := client.Connect(config.OBSConfig{Host: serverURL.Hostname(), Port: port}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	scenes, err := client.GetScenes()
	if err != nil {
		t.Fatalf("GetScenes: %v", err)
	}
	if len(scenes) != 1 || scenes[0].Name != "Main" {
		t.Fatalf("scenes = %#v", scenes)
	}
	sources, err := client.GetSources("Main")
	if err != nil {
		t.Fatalf("GetSources: %v", err)
	}
	if len(sources) != 2 || sources[0].SceneItemID != 10 {
		t.Fatalf("sources = %#v", sources)
	}
	if err := client.SetSourceVisibility("Main", "VTuber", 10, true); err != nil {
		t.Fatalf("SetSourceVisibility: %v", err)
	}
	if setVisibilityPayload["sceneName"] != "Main" || setVisibilityPayload["sceneItemId"].(float64) != 10 || setVisibilityPayload["sceneItemEnabled"] != true {
		t.Fatalf("visibility payload = %#v", setVisibilityPayload)
	}
}

func TestValidateHostRejectsRemoteByDefault(t *testing.T) {
	err := validateHost(config.OBSConfig{Host: "192.168.1.50", Port: 4455})
	if err == nil {
		t.Fatalf("expected remote host rejection")
	}
}

func TestValidateHostAllowsLoopbackAndExplicitRemote(t *testing.T) {
	if err := validateHost(config.OBSConfig{Host: "127.0.0.1", Port: 4455}); err != nil {
		t.Fatalf("loopback host rejected: %v", err)
	}
	if err := validateHost(config.OBSConfig{Host: "::1", Port: 4455}); err != nil {
		t.Fatalf("ipv6 loopback host rejected: %v", err)
	}
	if err := validateHost(config.OBSConfig{Host: "192.168.1.50", Port: 4455, AllowRemote: true}); err != nil {
		t.Fatalf("explicit remote host rejected: %v", err)
	}
}

func TestTransportSchemeUsesWSForLocalAndWSSForRemote(t *testing.T) {
	if got := transportScheme(config.OBSConfig{Host: "127.0.0.1", Port: 4455}); got != "ws" {
		t.Fatalf("local transport scheme = %q", got)
	}
	if got := transportScheme(config.OBSConfig{Host: "192.168.1.50", Port: 4455, AllowRemote: true}); got != "wss" {
		t.Fatalf("remote transport scheme = %q", got)
	}
}
