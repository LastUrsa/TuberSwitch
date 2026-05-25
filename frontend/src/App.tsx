import {type ReactNode, useEffect, useMemo, useState} from 'react';
import './App.css';
import tuberSwitchIcon from './assets/images/tuberswitch-icon.png';
import {BrowserOpenURL, LogError, LogInfo, WindowSetMinSize, WindowSetSize} from '../wailsjs/runtime/runtime';
import {
  ApplyMode,
  BrowseExecutable,
  CheckForUpdates,
  CreateTwitchReward,
  GetOBSInventory,
  GetStatus,
  GetTwitchRewards,
  ListRunningProcesses,
  RefreshTwitchRewards,
  SaveConfig,
  SetReward3DOnly,
  StartTwitchLogin,
  SyncOBS,
} from '../wailsjs/go/main/App';

type Mode = '3D' | 'PNG';

type Config = {
  obs: { host: string; port: number; allowRemote: boolean; passwordConfigured: boolean };
  sources: {
    scene: string;
    vtuberSource: string;
    vtuberItemId: number;
    pngTuberSource: string;
    pngTuberItemId: number;
  };
  sceneMappings: SceneMapping[];
  twitch: {
    clientId: string;
    channelId: string;
    channelName: string;
  };
  modeProfiles: unknown[];
  startupMode: 'restore-last' | 'always-3d' | 'always-png';
  currentMode: Mode;
  refreshRewardsOnStartup: boolean;
  appDetection: {
    enabled: boolean;
    threeDProcessName: string;
    pngProcessName: string;
    intervalSeconds: number;
    conflictBehavior: 'do-nothing' | 'prefer-3d' | 'prefer-png';
    applyTwitchChanges: boolean;
    manualOverrideCooldownSeconds: number;
  };
};

type SceneMapping = {
  scene: string;
  enabled: boolean;
  vtuberSource: string;
  vtuberItemId: number;
  pngTuberSource: string;
  pngTuberItemId: number;
};

type RewardMapping = {
  rewardId: string;
  rewardName: string;
  is3DOnly: boolean;
  manageable: boolean;
};

type Status = {
  config: Config;
  currentMode: Mode;
  currentModeLabel: string;
  obsConnected: boolean;
  twitchConnected: boolean;
  lastAction: string;
  appDetectionStatus: string;
  appDetectionEnabled: boolean;
};

type SettingsInput = {
  config: Config;
  obsPassword: string;
  updateObsPassword: boolean;
};

type ActionResult = {
  ok: boolean;
  message: string;
  warnings: string[];
  errors: string[];
  newStatus: Status;
};

type OBSInventory = {
  scenes: { name: string }[];
  sources: { name: string; sceneItemId: number }[];
  sourcesByScene: Record<string, { name: string; sceneItemId: number }[]>;
};

type ProcessSummary = {
  processName: string;
  pid: number;
};

type ProcessListOptions = {
  search: string;
  showOnlyVisibleApps: boolean;
  hideSystemProcesses: boolean;
  hideCommonDesktopApps: boolean;
  hideHelpersAndUtilities: boolean;
  likelyAvatarAppsOnly: boolean;
};

type TwitchReward = {
  id: string;
  title: string;
  enabled: boolean;
  is3DOnly: boolean;
  manageable: boolean;
};

type UpdateInfo = {
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  releaseUrl: string;
  message: string;
};

const emptyInventory: OBSInventory = {scenes: [], sources: [], sourcesByScene: {}};
const compactWindowSize = {width: 760, height: 460, minWidth: 680, minHeight: 420};
const settingsWindowSize = {width: 1080, height: 820, minWidth: 920, minHeight: 700};
type SettingsTab = 'general' | 'obs' | 'twitch';
type ThemeMode = 'dark' | 'light';
type ProcessFieldKey = 'threeDProcessName' | 'pngProcessName';
const themeStorageKey = 'tuberswitch-theme';

function App() {
  const [status, setStatus] = useState<Status | null>(null);
  const [draft, setDraft] = useState<Config | null>(null);
  const [inventory, setInventory] = useState<OBSInventory>(emptyInventory);
  const [rewards, setRewards] = useState<TwitchReward[]>([]);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [busy, setBusy] = useState('');
  const [errors, setErrors] = useState<string[]>([]);
  const [showSelectedScenesOnly, setShowSelectedScenesOnly] = useState(false);
  const [newRewardTitle, setNewRewardTitle] = useState('');
  const [newRewardCost, setNewRewardCost] = useState(1000);
  const [newRewardPrompt, setNewRewardPrompt] = useState('');
  const [scenesOpen, setScenesOpen] = useState(true);
  const [createRewardOpen, setCreateRewardOpen] = useState(false);
  const [manageableRewardsOpen, setManageableRewardsOpen] = useState(true);
  const [unmanageableRewardsOpen, setUnmanageableRewardsOpen] = useState(false);
  const [obsPassword, setObsPassword] = useState('');
  const [obsPasswordDirty, setObsPasswordDirty] = useState(false);
  const [settingsTab, setSettingsTab] = useState<SettingsTab>('general');
  const [processPickerField, setProcessPickerField] = useState<ProcessFieldKey | null>(null);
  const [theme, setTheme] = useState<ThemeMode>(() => {
    if (typeof window === 'undefined') return 'dark';
    const storedTheme = window.localStorage.getItem(themeStorageKey);
    return storedTheme === 'light' ? 'light' : 'dark';
  });
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [updateBusy, setUpdateBusy] = useState(false);

  useEffect(() => {
    load();
  }, []);

  useEffect(() => {
    const intervalID = window.setInterval(() => {
      void refreshStatus();
    }, 2000);
    return () => window.clearInterval(intervalID);
  }, [settingsOpen]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    window.localStorage.setItem(themeStorageKey, theme);
  }, [theme]);

  useEffect(() => {
    WindowSetMinSize(
      settingsOpen ? settingsWindowSize.minWidth : compactWindowSize.minWidth,
      settingsOpen ? settingsWindowSize.minHeight : compactWindowSize.minHeight,
    );
    WindowSetSize(
      settingsOpen ? settingsWindowSize.width : compactWindowSize.width,
      settingsOpen ? settingsWindowSize.height : compactWindowSize.height,
    );
  }, [settingsOpen]);

  useEffect(() => {
    if (!settingsOpen) return;

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        if (processPickerField) {
          setProcessPickerField(null);
          return;
        }
        closeSettings();
      }
    }

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [settingsOpen]);

  async function load() {
    const next = await GetStatus();
    setStatus(next as unknown as Status);
    setDraft(structuredClone((next as unknown as Status).config));
    setRewards((await GetTwitchRewards()) as TwitchReward[]);
    setObsPassword('');
    setObsPasswordDirty(false);
  }

  async function refreshStatus() {
    try {
      const next = await GetStatus();
      setStatus(next as unknown as Status);
      if (!settingsOpen) {
        setDraft(structuredClone((next as unknown as Status).config));
      }
    } catch {
      // Keep the current UI state if a background refresh misses once.
    }
  }

  async function run(label: string, action: () => Promise<ActionResult>) {
    setBusy(label);
    setErrors([]);
    try {
      const result = await action();
      setStatus(result.newStatus as unknown as Status);
      setDraft(structuredClone((result.newStatus as unknown as Status).config));
      setErrors(result.errors || []);
      setRewards((await GetTwitchRewards()) as TwitchReward[]);
      setObsPassword('');
      setObsPasswordDirty(false);
      return result;
    } catch (error) {
      setErrors([String(error)]);
    } finally {
      setBusy('');
    }
  }

  async function loadInventory(sceneName?: string) {
    setBusy('Loading OBS');
    setErrors([]);
    try {
      const nextInventory = await GetOBSInventory(sceneName || draft?.sources.scene || '') as OBSInventory;
      setInventory(nextInventory);
    } catch (error) {
      setErrors([String(error)]);
    } finally {
      setBusy('');
    }
  }

  async function saveSettings() {
    if (!draft) return;
    await run('Saving settings', () => SaveConfig(buildSettingsInput(draft, obsPassword, obsPasswordDirty) as never) as unknown as Promise<ActionResult>);
  }

  async function saveThen(label: string, action: () => Promise<ActionResult>) {
    if (!draft) return;
    setBusy('Saving settings');
    setErrors([]);
    try {
      const saved = await SaveConfig(buildSettingsInput(draft, obsPassword, obsPasswordDirty) as never) as unknown as ActionResult;
      setStatus(saved.newStatus as unknown as Status);
      setDraft(structuredClone((saved.newStatus as unknown as Status).config));
      setObsPassword('');
      setObsPasswordDirty(false);
      if (!saved.ok) {
        setErrors(saved.errors || [saved.message]);
        return saved;
      }
      return await run(label, action);
    } catch (error) {
      setErrors([String(error)]);
      setBusy('');
      return undefined;
    }
  }

  async function updateReward(rewardID: string, checked: boolean) {
    await run('Saving reward', () => SetReward3DOnly(rewardID, checked) as unknown as Promise<ActionResult>);
  }

  async function createReward() {
    const result = await saveThen('Creating reward', () => CreateTwitchReward(newRewardTitle, newRewardCost, newRewardPrompt) as unknown as Promise<ActionResult>);
    if (result?.ok) {
      setNewRewardTitle('');
      setNewRewardPrompt('');
      setRewards((await GetTwitchRewards()) as TwitchReward[]);
    }
  }

  function updateSceneMapping(index: number, patch: Partial<SceneMapping>) {
    if (!draft) return;
    const sceneMappings = [...(draft.sceneMappings || [])];
    sceneMappings[index] = {...sceneMappings[index], ...patch};
    setDraft({...draft, sceneMappings});
  }

  function openSettings() {
    setSettingsTab('general');
    setSettingsOpen(true);
  }

  function closeSettings() {
    setProcessPickerField(null);
    setSettingsOpen(false);
  }

  function setAppDetectionProcessName(field: ProcessFieldKey, value: string) {
    if (!draft) return;
    setDraft({
      ...draft,
      appDetection: {
        ...draft.appDetection,
        [field]: normalizeExecutableName(value),
      },
    });
  }

  async function browseExecutableFor(field: ProcessFieldKey) {
    try {
      const filename = normalizeExecutableName(await BrowseExecutable());
      if (!filename) return;
      setAppDetectionProcessName(field, filename);
      LogInfo(`App detection browse selected filename: ${filename}`);
    } catch (error) {
      LogError(`App detection executable browse failed: ${String(error)}`);
      setErrors([String(error)]);
    }
  }

  async function checkForUpdates() {
    setUpdateBusy(true);
    setErrors([]);
    try {
      const result = await CheckForUpdates();
      setUpdateInfo(result as unknown as UpdateInfo);
    } catch (error) {
      setErrors([`Update check failed: ${String(error)}`]);
    } finally {
      setUpdateBusy(false);
    }
  }

  const is3D = status?.currentMode === '3D';
  const currentMode = status?.currentModeLabel || 'PNG VTuber Mode';
  const canInteract = !busy && status && draft;
  const visibleSceneMappings = (draft?.sceneMappings || [])
    .map((mapping, index) => ({mapping, index}))
    .filter(({mapping}) => !showSelectedScenesOnly || mapping.enabled);
  const manageableRewards = rewards.filter((reward) => reward.manageable);
  const unmanageableRewards = rewards.filter((reward) => !reward.manageable);

  return (
    <main className="app-shell">
      <section className="topbar">
        <div className="brand-block">
          <img className="brand-icon" src={tuberSwitchIcon} alt="TuberSwitch icon" />
          <div>
            <h1>TuberSwitch</h1>
            <p>{currentMode}</p>
          </div>
        </div>
        <div className="topbar-actions">
          <div className="connection-strip" aria-label="Connection status">
            <ConnectionStatus
              label="OBS"
              connected={!!status?.obsConnected}
            />
            <ConnectionStatus
              label="Twitch"
              connected={!!status?.twitchConnected}
            />
          </div>
          <button className="secondary" onClick={settingsOpen ? closeSettings : openSettings}>
            Settings
          </button>
        </div>
      </section>

      <section className="mode-panel">
        <div className="mode-copy">
          <span>Current Mode</span>
          <strong>{currentMode}</strong>
        </div>
        <button
          className={`mode-switch ${is3D ? 'on' : 'off'}`}
          disabled={!canInteract}
          onClick={() => run(is3D ? 'Switching to PNG' : 'Switching to 3D', () => ApplyMode(is3D ? 'PNG' : '3D') as unknown as Promise<ActionResult>)}
          aria-pressed={is3D}
        >
          <span>{is3D ? '3D' : 'PNG'}</span>
          <b>{is3D ? '3D VTuber Mode' : 'PNG VTuber Mode'}</b>
        </button>
      </section>

      {errors.length > 0 && (
        <section className="error-list">
          {errors.map((error) => <div key={error}>{error}</div>)}
        </section>
      )}

      {settingsOpen && draft && (
        <div className="settings-modal-backdrop" onClick={closeSettings}>
          <section
            className="settings-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="settings-title"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="settings-modal-header">
              <div>
                <h2 id="settings-title">Settings</h2>
                <p>Configure OBS, Twitch, and reward behavior.</p>
              </div>
              <button type="button" className="secondary" onClick={closeSettings}>
                Close
              </button>
            </div>

            <section className="modal-layout">
              <div className="settings-tabs" role="tablist" aria-label="Settings sections">
                <button
                  type="button"
                  role="tab"
                  className={`settings-tab ${settingsTab === 'general' ? 'active' : ''}`}
                  aria-selected={settingsTab === 'general'}
                  onClick={() => setSettingsTab('general')}
                >
                  General
                </button>
                <button
                  type="button"
                  role="tab"
                  className={`settings-tab ${settingsTab === 'obs' ? 'active' : ''}`}
                  aria-selected={settingsTab === 'obs'}
                  onClick={() => setSettingsTab('obs')}
                >
                  OBS Settings
                </button>
                <button
                  type="button"
                  role="tab"
                  className={`settings-tab ${settingsTab === 'twitch' ? 'active' : ''}`}
                  aria-selected={settingsTab === 'twitch'}
                  onClick={() => setSettingsTab('twitch')}
                >
                  Twitch Settings
                </button>
              </div>

              {settingsTab === 'general' && (
                <section className="settings-panel">
                  <div className="settings-column">
                    <h2>General Settings</h2>
                    <div className="form-grid">
                      <label>
                        <FieldLabel text="Theme"/>
                        <select value={theme} onChange={(event) => setTheme(event.currentTarget.value as ThemeMode)}>
                          <option value="dark">Dark</option>
                          <option value="light">Light</option>
                        </select>
                      </label>
                      <section className="inline-settings-section">
                        <h3>App Detection</h3>
                        <label className="check-row">
                          <input
                            type="checkbox"
                            checked={draft.appDetection.enabled}
                            onChange={(event) => setDraft({...draft, appDetection: {...draft.appDetection, enabled: event.currentTarget.checked}})}
                          />
                          <span>Enable App Detection</span>
                        </label>
                        <ProcessNameField
                          label="3D Mode Process"
                          info={(
                            <>
                              <p>Type the executable name manually, select a running app, or browse to an executable.</p>
                            </>
                          )}
                          value={draft.appDetection.threeDProcessName}
                          onChange={(value) => setAppDetectionProcessName('threeDProcessName', value)}
                          onSelectRunningApp={() => setProcessPickerField('threeDProcessName')}
                          onBrowseExecutable={() => void browseExecutableFor('threeDProcessName')}
                        />
                        <ProcessNameField
                          label="PNG Mode Process"
                          info={(
                            <>
                              <p>TuberSwitch stores only the executable filename, such as <code>AvatarApp.exe</code>.</p>
                            </>
                          )}
                          value={draft.appDetection.pngProcessName}
                          onChange={(value) => setAppDetectionProcessName('pngProcessName', value)}
                          onSelectRunningApp={() => setProcessPickerField('pngProcessName')}
                          onBrowseExecutable={() => void browseExecutableFor('pngProcessName')}
                        />
                        <NumberInput
                          label="Detection Interval (seconds)"
                          value={draft.appDetection.intervalSeconds}
                          min={2}
                          onChange={(value) => setDraft({...draft, appDetection: {...draft.appDetection, intervalSeconds: value}})}
                        />
                        <label>
                          <FieldLabel text="Conflict Behavior"/>
                          <select
                            value={draft.appDetection.conflictBehavior}
                            onChange={(event) => setDraft({...draft, appDetection: {...draft.appDetection, conflictBehavior: event.currentTarget.value as Config['appDetection']['conflictBehavior']}})}
                          >
                            <option value="do-nothing">Do Nothing</option>
                            <option value="prefer-3d">Prefer 3D</option>
                            <option value="prefer-png">Prefer PNG</option>
                          </select>
                        </label>
                        <NumberInput
                          label="Manual Override Cooldown (seconds)"
                          value={draft.appDetection.manualOverrideCooldownSeconds}
                          min={0}
                          onChange={(value) => setDraft({...draft, appDetection: {...draft.appDetection, manualOverrideCooldownSeconds: value}})}
                        />
                        <label className="check-row">
                          <input
                            type="checkbox"
                            checked={draft.appDetection.applyTwitchChanges}
                            onChange={(event) => setDraft({...draft, appDetection: {...draft.appDetection, applyTwitchChanges: event.currentTarget.checked}})}
                          />
                          <span>Apply Twitch changes during auto-switch</span>
                        </label>
                        <div className="settings-note">
                          <span>Current detection status</span>
                          <strong>{status?.appDetectionStatus || 'Disabled'}</strong>
                        </div>
                      </section>
                    </div>
                    <div className="settings-note">
                      <span>Current version</span>
                      <strong>{updateInfo?.currentVersion || '0.3.0'}</strong>
                    </div>
                    <div className="button-row">
                      <button type="button" onClick={saveSettings} disabled={!!busy}>Save</button>
                      <button type="button" onClick={checkForUpdates} disabled={updateBusy}>
                        {updateBusy ? 'Checking for Updates' : 'Check for Updates'}
                      </button>
                      {updateInfo?.updateAvailable && (
                        <button type="button" className="secondary" onClick={() => BrowserOpenURL(updateInfo.releaseUrl)}>
                          Open GitHub Releases
                        </button>
                      )}
                    </div>
                    {updateInfo && (
                      <div className={`update-panel ${updateInfo.updateAvailable ? 'available' : 'current'}`}>
                        <strong>{updateInfo.message}</strong>
                        <span>Latest version: {updateInfo.latestVersion}</span>
                      </div>
                    )}
                  </div>
                </section>
              )}

              {settingsTab === 'obs' && (
                <section className="settings-panel">
                  <div className="settings-column">
                    <h2>OBS Settings</h2>
                    <div className="form-grid settings-form-grid">
                      <TextInput
                        label="OBS WebSocket Host"
                        info={(
                          <>
                            <p>Set this from <strong>Tools &gt; WebSocket Server Settings</strong> in OBS.</p>
                            <p>The usual local default is <code>127.0.0.1</code>.</p>
                          </>
                        )}
                        value={draft.obs.host}
                        onChange={(value) => setDraft({...draft, obs: {...draft.obs, host: value}})}
                      />
                      <NumberInput
                        label="OBS WebSocket Port"
                        info={(
                          <>
                            <p>Use the port shown in <strong>Tools &gt; WebSocket Server Settings</strong> in OBS.</p>
                          </>
                        )}
                        value={draft.obs.port}
                        onChange={(value) => setDraft({...draft, obs: {...draft.obs, port: value}})}
                      />
                      <TextInput
                        label="OBS WebSocket Password"
                        info={(
                          <>
                            <p>Use the password from <strong>Tools &gt; WebSocket Server Settings</strong> in OBS.</p>
                            <p>Stored securely in the OS credential store and never shown back in the app.</p>
                          </>
                        )}
                        type="password"
                        value={obsPassword}
                        placeholder={draft.obs.passwordConfigured ? 'Saved securely. Enter a new value to replace it.' : 'Enter OBS WebSocket password'}
                        onChange={(value) => {
                          setObsPassword(value);
                          setObsPasswordDirty(true);
                        }}
                      />
                      <label className="check-row settings-checkbox-row">
                        <input
                          type="checkbox"
                          checked={draft.obs.allowRemote}
                          onChange={(event) => setDraft({...draft, obs: {...draft.obs, allowRemote: event.currentTarget.checked}})}
                        />
                        <span>Allow remote OBS connections</span>
                      </label>
                    </div>
                    <div className="button-row">
                      <button onClick={saveSettings} disabled={!!busy}>Save</button>
                      <button onClick={async () => {
                        const result = await saveThen('Syncing OBS', () => SyncOBS() as unknown as Promise<ActionResult>);
                        await loadInventory(result?.newStatus?.config?.sceneMappings?.[0]?.scene || '');
                      }} disabled={!!busy}>Sync Scenes & Sources</button>
                    </div>

                    <CollapsibleSection title="Scenes" open={scenesOpen} onToggle={setScenesOpen}>
                      <div className="mapping-toolbar">
                        <label className="check-row">
                          <input
                            type="checkbox"
                            checked={showSelectedScenesOnly}
                            onChange={(event) => setShowSelectedScenesOnly(event.currentTarget.checked)}
                          />
                          <span>Show only selected scenes</span>
                        </label>
                        <span>{visibleSceneMappings.length} of {(draft.sceneMappings || []).length} scenes shown</span>
                      </div>

                      <div className="scene-mapping-table">
                        <div className="scene-mapping-head">
                          <span>Use</span>
                          <span>Scene</span>
                          <span>VTuber Source</span>
                          <span>PNG Tuber Source</span>
                        </div>
                        {(draft.sceneMappings || []).length === 0 && <div className="empty-row">Sync OBS to load scenes</div>}
                        {visibleSceneMappings.map(({mapping, index}) => {
                          const sources = inventory.sourcesByScene?.[mapping.scene] || [];
                          const missingSource = mapping.enabled && (!mapping.vtuberSource || !mapping.pngTuberSource);
                          return (
                            <div className="scene-mapping-row" key={mapping.scene || index}>
                              <input
                                type="checkbox"
                                checked={mapping.enabled}
                                onChange={(event) => updateSceneMapping(index, {enabled: event.currentTarget.checked})}
                              />
                              <strong className="scene-name">
                                {missingSource && <span className="warning-icon" title="Only selected sources in this scene will be toggled">!</span>}
                                <span>{mapping.scene}</span>
                              </strong>
                              <SourceSelect
                                label="VTuber Source"
                                value={mapping.vtuberSource}
                                sources={sources}
                                onChange={(name, id) => updateSceneMapping(index, {vtuberSource: name, vtuberItemId: id, enabled: true})}
                              />
                              <SourceSelect
                                label="PNG Tuber Source"
                                value={mapping.pngTuberSource}
                                sources={sources}
                                onChange={(name, id) => updateSceneMapping(index, {pngTuberSource: name, pngTuberItemId: id, enabled: true})}
                              />
                            </div>
                          );
                        })}
                      </div>
                    </CollapsibleSection>

                  </div>
                </section>
              )}

              {settingsTab === 'twitch' && (
                <section className="settings-panel">
                  <div className="settings-column">
                    <h2>Twitch Settings</h2>
                    <div className="form-grid">
                      <TextInput
                        label="Twitch Client ID"
                        info={(
                          <>
                            <p>Create your app in the <a href="https://dev.twitch.tv/console/apps" target="_blank" rel="noreferrer">Twitch Developer Console</a>.</p>
                            <p>Use a public app, request <code>channel:read:redemptions</code> and <code>channel:manage:redemptions</code>, and use <code>https://localhost</code> if Twitch insists on a redirect URI.</p>
                            <p>Copy the Client ID from the app details page into this field.</p>
                          </>
                        )}
                        type="password"
                        value={draft.twitch.clientId}
                        placeholder="Enter Twitch Client ID"
                        onChange={(value) => setDraft({...draft, twitch: {...draft.twitch, clientId: value}})}
                      />
                      <label className="check-row">
                        <input
                          type="checkbox"
                          checked={draft.refreshRewardsOnStartup}
                          onChange={(event) => setDraft({...draft, refreshRewardsOnStartup: event.currentTarget.checked})}
                        />
                        <span>Refresh Twitch rewards on startup</span>
                      </label>
                      <label>
                        <FieldLabel text="Startup Mode"/>
                        <select value={draft.startupMode} onChange={(event) => setDraft({...draft, startupMode: event.currentTarget.value as Config['startupMode']})}>
                          <option value="restore-last">Restore Last Mode</option>
                          <option value="always-3d">Always 3D</option>
                          <option value="always-png">Always PNG</option>
                        </select>
                      </label>
                    </div>
                    <div className="channel-name">
                      Authenticated Channel: <strong>{draft.twitch.channelName || 'Not connected'}</strong>
                    </div>
                    <div className="button-row">
                      <button onClick={saveSettings} disabled={!!busy}>Save</button>
                      <button onClick={() => saveThen('Logging in', () => StartTwitchLogin() as unknown as Promise<ActionResult>)} disabled={!!busy}>Login with Twitch</button>
                      <button onClick={() => saveThen('Refreshing rewards', () => RefreshTwitchRewards() as unknown as Promise<ActionResult>)} disabled={!!busy}>Refresh Rewards</button>
                    </div>

                    <CollapsibleSection title="Create Reward" open={createRewardOpen} onToggle={setCreateRewardOpen}>
                      <div className="create-reward">
                        <TextInput label="New Reward Name" value={newRewardTitle} onChange={setNewRewardTitle}/>
                        <NumberInput label="Cost" value={newRewardCost} onChange={setNewRewardCost}/>
                        <TextInput label="Prompt (optional)" value={newRewardPrompt} onChange={setNewRewardPrompt}/>
                        <button onClick={createReward} disabled={!!busy || !newRewardTitle.trim()}>Create Reward</button>
                      </div>
                    </CollapsibleSection>

                    <CollapsibleSection title="Manageable Rewards" open={manageableRewardsOpen} onToggle={setManageableRewardsOpen}>
                      <div className="reward-table">
                        <div className="reward-head">
                          <span>Reward Name</span>
                          <span>3D Only</span>
                        </div>
                        {manageableRewards.length === 0 && <div className="empty-row">No manageable rewards loaded</div>}
                        {manageableRewards.map((reward) => (
                          <label className="reward-row" key={reward.id}>
                            <span>{reward.title}</span>
                            <input type="checkbox" checked={reward.is3DOnly} onChange={(event) => updateReward(reward.id, event.currentTarget.checked)}/>
                          </label>
                        ))}
                      </div>
                    </CollapsibleSection>

                    <CollapsibleSection title="Unmanageable Rewards" open={unmanageableRewardsOpen} onToggle={setUnmanageableRewardsOpen}>
                      <div className="reward-table">
                        <div className="reward-head readonly">
                          <span>Reward Name</span>
                          <span>Status</span>
                        </div>
                        {unmanageableRewards.length === 0 && <div className="empty-row">No unmanageable rewards loaded</div>}
                        {unmanageableRewards.map((reward) => (
                          <div className="reward-row readonly" key={reward.id}>
                            <span>{reward.title}</span>
                            <span>Read-only</span>
                          </div>
                        ))}
                      </div>
                    </CollapsibleSection>
                  </div>
                </section>
              )}
            </section>
          </section>
          <ProcessPickerDialog
            open={!!processPickerField}
            title={processPickerField === 'pngProcessName' ? 'Select PNG Mode Process' : 'Select 3D Mode Process'}
            onClose={() => setProcessPickerField(null)}
            onSelect={(process) => {
              if (!processPickerField) return;
              const normalizedName = normalizeExecutableName(process.processName);
              setAppDetectionProcessName(processPickerField, normalizedName);
              LogInfo(`App detection selected process name: ${normalizedName}`);
              setProcessPickerField(null);
            }}
          />
        </div>
      )}
    </main>
  );
}

function ConnectionStatus({label, connected}: {label: string; connected: boolean}) {
  return (
    <span
      className="connection-pill"
      title={`${label} ${connected ? 'connected' : 'disconnected'}`}
      aria-label={`${label} ${connected ? 'connected' : 'disconnected'}`}
    >
      <span className={`connection-dot ${connected ? 'connected' : 'disconnected'}`} aria-hidden="true" />
      <span>{label}</span>
    </span>
  );
}

function TextInput({label, value, onChange, type = 'text', info, placeholder}: {label: string; value: string; onChange: (value: string) => void; type?: string; info?: ReactNode; placeholder?: string}) {
  return (
    <label>
      <FieldLabel text={label} info={info}/>
      <input type={type} value={value || ''} placeholder={placeholder} onChange={(event) => onChange(event.currentTarget.value)}/>
    </label>
  );
}

function ProcessNameField({
  label,
  value,
  onChange,
  onSelectRunningApp,
  onBrowseExecutable,
  info,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  onSelectRunningApp: () => void;
  onBrowseExecutable: () => void;
  info?: ReactNode;
}) {
  return (
    <div className="process-input-field">
      <label>
        <FieldLabel text={label} info={info}/>
        <input type="text" value={value || ''} onChange={(event) => onChange(event.currentTarget.value)}/>
      </label>
      <div className="process-input-actions">
        <button type="button" className="process-action-button" onClick={onSelectRunningApp}>Select Running App</button>
        <button type="button" className="process-action-button" onClick={onBrowseExecutable}>Browse Executable</button>
      </div>
    </div>
  );
}

function NumberInput({label, value, onChange, info, min}: {label: string; value: number; onChange: (value: number) => void; info?: ReactNode; min?: number}) {
  return (
    <label>
      <FieldLabel text={label} info={info}/>
      <input type="number" min={min} value={value || 0} onChange={(event) => onChange(Number(event.currentTarget.value))}/>
    </label>
  );
}

function FieldLabel({text, info}: {text: string; info?: ReactNode}) {
  return (
    <span className="label-row">
      {text}
      {info && <InfoTip>{info}</InfoTip>}
    </span>
  );
}

function SourceSelect({label, value, sources, onChange}: {label: string; value: string; sources: { name: string; sceneItemId: number }[]; onChange: (name: string, id: number) => void}) {
  const values = useMemo(() => {
    const existing = sources.some((source) => source.name === value);
    return existing || !value ? sources : [{name: value, sceneItemId: 0}, ...sources];
  }, [sources, value]);
  return (
    <label>
      <FieldLabel text={label}/>
      <select
        value={value || ''}
        onChange={(event) => {
          const source = sources.find((item) => item.name === event.currentTarget.value);
          onChange(event.currentTarget.value, source?.sceneItemId || 0);
        }}
      >
        <option value="">Select...</option>
        {values.map((source) => <option key={`${source.name}-${source.sceneItemId}`} value={source.name}>{source.name}</option>)}
      </select>
    </label>
  );
}

function InfoTip({children}: {children: ReactNode}) {
  return (
    <span className="info-tip" tabIndex={0} aria-label="More information">
      ?
      <span className="info-bubble">{children}</span>
    </span>
  );
}

function CollapsibleSection({title, open, onToggle, children}: {title: string; open: boolean; onToggle: (open: boolean) => void; children: ReactNode}) {
  return (
    <section className="collapsible-section">
      <button
        type="button"
        className="collapsible-toggle secondary"
        onClick={() => onToggle(!open)}
        aria-expanded={open}
      >
        <span>{title}</span>
        <span>{open ? 'Hide' : 'Show'}</span>
      </button>
      {open && <div className="collapsible-body">{children}</div>}
    </section>
  );
}

function ProcessPickerDialog({
  open,
  onClose,
  onSelect,
  title,
}: {
  open: boolean;
  onClose: () => void;
  onSelect: (process: ProcessSummary) => void;
  title: string;
}) {
  const [processes, setProcesses] = useState<ProcessSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [likelyAvatarAppsOnly, setLikelyAvatarAppsOnly] = useState(true);
  const [showOnlyVisibleApps, setShowOnlyVisibleApps] = useState(true);
  const [hideSystemProcesses, setHideSystemProcesses] = useState(true);
  const [hideCommonApps, setHideCommonApps] = useState(true);
  const [hideHelpersAndUtilities, setHideHelpersAndUtilities] = useState(true);
  const [selectedKey, setSelectedKey] = useState('');

  async function loadProcesses(options: ProcessListOptions) {
    setLoading(true);
    setError('');
    try {
      const nextProcesses = await ListRunningProcesses(options as never) as ProcessSummary[];
      setProcesses(nextProcesses);
    } catch (loadError) {
      setError(String(loadError));
      LogError(`App detection process list failed: ${String(loadError)}`);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (!open) return;
    setQuery('');
    setSelectedKey('');
    setLikelyAvatarAppsOnly(true);
    setShowOnlyVisibleApps(true);
    setHideSystemProcesses(true);
    setHideCommonApps(true);
    setHideHelpersAndUtilities(true);
    LogInfo(`App detection process picker opened: ${title}`);
  }, [open, title]);

  const options = useMemo(() => ({
    search: query,
    likelyAvatarAppsOnly,
    showOnlyVisibleApps,
    hideSystemProcesses,
    hideCommonDesktopApps: hideCommonApps,
    hideHelpersAndUtilities,
  }), [hideCommonApps, hideHelpersAndUtilities, hideSystemProcesses, likelyAvatarAppsOnly, query, showOnlyVisibleApps]);

  useEffect(() => {
    if (!open) return;
    void loadProcesses(options);
  }, [open, options]);

  const selectedProcess = processes.find((process) => processKey(process) === selectedKey) || null;

  if (!open) return null;

  return (
    <div className="dialog-backdrop" onClick={onClose}>
      <section
        className="process-picker-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="process-picker-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="process-picker-header">
          <div>
            <h3 id="process-picker-title">{title}</h3>
            <p>Select a running process to copy its executable name into this field.</p>
          </div>
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
        </div>
        <div className="process-picker-toolbar">
          <label>
            <FieldLabel text="Search"/>
            <input
              type="text"
              value={query}
              placeholder="Filter by process name"
              onChange={(event) => setQuery(event.currentTarget.value)}
            />
          </label>
          <label className="check-row">
            <input
              type="checkbox"
              checked={likelyAvatarAppsOnly}
              onChange={(event) => setLikelyAvatarAppsOnly(event.currentTarget.checked)}
            />
            <span>Likely avatar apps only</span>
          </label>
          <label className="check-row">
            <input
              type="checkbox"
              checked={showOnlyVisibleApps}
              onChange={(event) => setShowOnlyVisibleApps(event.currentTarget.checked)}
            />
            <span>Show only visible app windows</span>
          </label>
          <label className="check-row">
            <input
              type="checkbox"
              checked={hideSystemProcesses}
              onChange={(event) => setHideSystemProcesses(event.currentTarget.checked)}
            />
            <span>Hide system processes</span>
          </label>
          <label className="check-row">
            <input
              type="checkbox"
              checked={hideCommonApps}
              onChange={(event) => setHideCommonApps(event.currentTarget.checked)}
            />
            <span>Hide common desktop apps</span>
          </label>
          <label className="check-row">
            <input
              type="checkbox"
              checked={hideHelpersAndUtilities}
              onChange={(event) => setHideHelpersAndUtilities(event.currentTarget.checked)}
            />
            <span>Hide helpers and utilities</span>
          </label>
          <button type="button" className="secondary" onClick={() => void loadProcesses(options)} disabled={loading}>Refresh</button>
        </div>
        {loading && <div className="process-picker-state">Loading running processes...</div>}
        {error && <div className="process-picker-state error">{error}</div>}
        {!loading && !error && (
          <div className="process-picker-select-wrap">
            {processes.length === 0 ? (
              <div className="process-picker-state">No running processes match the current filter.</div>
            ) : (
              <label>
                <FieldLabel text="Running App"/>
                <select value={selectedKey} onChange={(event) => setSelectedKey(event.currentTarget.value)}>
                  <option value="">Select a process...</option>
                  {processes.map((process) => (
                    <option key={processKey(process)} value={processKey(process)}>
                      {formatProcessOption(process)}
                    </option>
                  ))}
                </select>
              </label>
            )}
          </div>
        )}
        <div className="button-row process-picker-actions">
          <button type="button" onClick={() => selectedProcess && onSelect(selectedProcess)} disabled={!selectedProcess}>Select</button>
          <button type="button" className="secondary" onClick={onClose}>Cancel</button>
        </div>
      </section>
    </div>
  );
}

function buildSettingsInput(config: Config, obsPassword: string, updateObsPassword: boolean): SettingsInput {
  return {
    config,
    obsPassword,
    updateObsPassword,
  };
}

function normalizeExecutableName(value: string) {
  const trimmedValue = value.trim();
  if (!trimmedValue) return '';
  const segments = trimmedValue.split(/[\\/]/);
  return segments[segments.length - 1].trim();
}

function processKey(process: ProcessSummary) {
  return `${process.processName}:${process.pid}`;
}

function formatProcessOption(process: ProcessSummary) {
  return `${process.processName || 'Unknown process'} (PID ${process.pid})`;
}

export default App;
