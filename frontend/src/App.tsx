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
  DeleteProfile,
  SaveConfig,
  SaveProfile,
  SaveProfileAs,
  SelectProfile,
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
  rewardMappings: RewardMapping[];
  profiles: Profile[];
  activeProfileId: string;
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

type Profile = {
  id: string;
  name: string;
  mode: Mode;
  sources: Config['sources'];
  sceneMappings: SceneMapping[];
  rewardMappings: RewardMapping[];
  lastUsed: string;
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
const compactWindowSize = {width: 920, height: 580, minWidth: 860, minHeight: 540};
const settingsWindowSize = {width: 1200, height: 840, minWidth: 1040, minHeight: 720};
type SettingsTab = 'general' | 'connections' | 'profiles' | 'about';
type ThemeMode = 'dark' | 'light';
type ProcessFieldKey = 'threeDProcessName' | 'pngProcessName';
type ProfileDialog = 'save-as' | 'delete' | null;
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
  const [profileDialog, setProfileDialog] = useState<ProfileDialog>(null);
  const [profileNameInput, setProfileNameInput] = useState('');
  const [profileDialogError, setProfileDialogError] = useState('');
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
        if (profileDialog) {
          closeProfileDialog();
          return;
        }
        if (processPickerField) {
          setProcessPickerField(null);
          return;
        }
        closeSettings();
      }
    }

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [settingsOpen, processPickerField, profileDialog]);

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

  async function saveCurrentProfile() {
    if (!draft) return;
    await run('Saving profile', () => SaveProfile(buildSettingsInput(draft, obsPassword, obsPasswordDirty) as never) as unknown as Promise<ActionResult>);
  }

  async function saveCurrentProfileAs() {
    if (!draft) return;
    setProfileNameInput('');
    setProfileDialogError('');
    setProfileDialog('save-as');
  }

  async function confirmSaveProfileAs() {
    if (!draft) return;
    const trimmedName = profileNameInput.trim();
    if (!trimmedName) {
      setProfileDialogError('Profile name is required.');
      return;
    }
    closeProfileDialog();
    await run('Saving profile', () => SaveProfileAs(trimmedName, buildSettingsInput(draft, obsPassword, obsPasswordDirty) as never) as unknown as Promise<ActionResult>);
  }

  async function deleteCurrentProfile() {
    if (!draft) return;
    const activeProfile = findActiveProfile(draft);
    if (!activeProfile || activeProfile.id === 'default') return;
    setProfileDialogError('');
    setProfileDialog('delete');
  }

  async function confirmDeleteCurrentProfile() {
    if (!draft) return;
    const activeProfile = findActiveProfile(draft);
    if (!activeProfile || activeProfile.id === 'default') return;
    closeProfileDialog();
    await run('Deleting profile', () => DeleteProfile(activeProfile.id) as unknown as Promise<ActionResult>);
  }

  async function selectProfile(profileID: string) {
    if (!draft || profileID === draft.activeProfileId) return;
    await run('Applying profile', () => SelectProfile(profileID) as unknown as Promise<ActionResult>);
  }

  function closeProfileDialog() {
    setProfileDialog(null);
    setProfileDialogError('');
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

  async function saveProfileThen(label: string, action: () => Promise<ActionResult>) {
    if (!draft) return;
    setBusy('Saving profile');
    setErrors([]);
    try {
      const saved = await SaveProfile(buildSettingsInput(draft, obsPassword, obsPasswordDirty) as never) as unknown as ActionResult;
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

  function updateDesiredSceneSource(index: number, name: string, id: number) {
    if (!draft) return;
    const patch = draft.currentMode === '3D'
      ? {vtuberSource: name, vtuberItemId: id, enabled: true}
      : {pngTuberSource: name, pngTuberItemId: id, enabled: true};
    updateSceneMapping(index, patch);
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
  const currentMode = status?.currentModeLabel || 'PNGTuber Mode';
  const canInteract = !busy && status && draft;
  const currentVersion = updateInfo?.currentVersion || '0.5.0';
  const nextModeLabel = is3D ? 'Switch to PNGTuber mode' : 'Switch to 3D VTuber mode';
  const nextModeShortLabel = is3D ? 'PNGTuber' : '3D';
  const obsHostInvalid = !!draft && !draft.obs.host.trim();
  const obsPortInvalid = !!draft && draft.obs.port < 1;
  const twitchClientIdInvalid = !!draft && !draft.twitch.clientId.trim();
  const appDetectionIntervalInvalid = !!draft && draft.appDetection.intervalSeconds < 2;
  const visibleSceneMappings = (draft?.sceneMappings || [])
    .map((mapping, index) => ({mapping, index}))
    .filter(({mapping}) => !showSelectedScenesOnly || mapping.enabled);
  const manageableRewards = rewards.filter((reward) => reward.manageable);
  const unmanageableRewards = rewards.filter((reward) => !reward.manageable);
  const activeProfile = draft ? findActiveProfile(draft) : null;
  const profileDirty = !!draft && !!activeProfile && !profileMatchesDraft(activeProfile, draft);
  const orderedProfiles = orderProfiles(draft?.profiles || []);
  const recentProfiles = orderedProfiles.filter((profile) => profile.lastUsed);
  const remainingProfiles = orderedProfiles.filter((profile) => !profile.lastUsed);

  return (
    <main className="app-shell">
      <section className="topbar">
        <div className="brand-block">
          <img className="brand-icon" src={tuberSwitchIcon} alt="TuberSwitch icon" />
          <div className="brand-copy">
            <span className="suite-eyebrow">Starsong Tools</span>
            <h1>TuberSwitch</h1>
            <p>Avatar mode control for OBS scenes and Twitch reward behavior.</p>
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
          <button className="secondary icon-only-button" onClick={settingsOpen ? closeSettings : openSettings} aria-label="Open settings" title="Open settings">
            <GearIcon/>
          </button>
        </div>
      </section>

      <section className="mode-panel">
        <div className="mode-copy">
          <span className="panel-eyebrow">Active presentation mode</span>
          <strong>{currentMode}</strong>
        </div>
        <div className="mode-control-row">
          {draft && (
            <div className="profile-inline" aria-labelledby="profile-title">
              <div className="profile-inline-head">
                <span id="profile-title">Profile</span>
              </div>
              <div className="profile-inline-controls">
                <label className="profile-select-label">
                  <span className="sr-only">Active profile</span>
                  <select
                    value={draft.activeProfileId || 'default'}
                    onChange={(event) => void selectProfile(event.currentTarget.value)}
                    disabled={!canInteract}
                    aria-label="Active profile"
                  >
                    {recentProfiles.length > 0 && (
                      <optgroup label="Recently Used">
                        {recentProfiles.map((profile) => (
                          <option key={profile.id} value={profile.id}>{profile.name}</option>
                        ))}
                      </optgroup>
                    )}
                    <optgroup label="All Profiles">
                      {remainingProfiles.map((profile) => (
                        <option key={profile.id} value={profile.id}>{profile.name}</option>
                      ))}
                      {recentProfiles.map((profile) => (
                        <option key={`all-${profile.id}`} value={profile.id}>{profile.name}</option>
                      ))}
                    </optgroup>
                  </select>
                </label>
              </div>
            </div>
          )}
          <div className="mode-actions">
            <button
              className={`mode-switch ${is3D ? 'on' : 'off'}`}
              disabled={!canInteract}
              onClick={() => run(is3D ? 'Switching to PNG' : 'Switching to 3D', () => ApplyMode(is3D ? 'PNG' : '3D') as unknown as Promise<ActionResult>)}
              aria-label={nextModeLabel}
              aria-pressed={is3D}
              title={nextModeLabel}
            >
              <span aria-hidden="true"><SwitchIcon/></span>
              <b>{nextModeShortLabel}</b>
            </button>
          </div>
        </div>
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
              <div className="settings-modal-copy">
                <span className="suite-eyebrow">Settings workspace</span>
                <h2 id="settings-title">Settings</h2>
                <p>Configure automation, OBS scene control, and Twitch reward behavior for this Starsong utility.</p>
              </div>
              <button type="button" className="secondary icon-only-button" onClick={closeSettings} aria-label="Close settings">
                <CloseIcon/>
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
                  <span className="settings-tab-title">General</span>
                  <small>Theme and automation</small>
                </button>
                <button
                  type="button"
                  role="tab"
                  className={`settings-tab ${settingsTab === 'connections' ? 'active' : ''}`}
                  aria-selected={settingsTab === 'connections'}
                  onClick={() => setSettingsTab('connections')}
                >
                  <span className="settings-tab-title">Connections</span>
                  <small>OBS and Twitch auth</small>
                </button>
                <button
                  type="button"
                  role="tab"
                  className={`settings-tab ${settingsTab === 'profiles' ? 'active' : ''}`}
                  aria-selected={settingsTab === 'profiles'}
                  onClick={() => setSettingsTab('profiles')}
                >
                  <span className="settings-tab-title">Profiles</span>
                  <small>Modes, scenes, rewards</small>
                </button>
                <button
                  type="button"
                  role="tab"
                  className={`settings-tab ${settingsTab === 'about' ? 'active' : ''}`}
                  aria-selected={settingsTab === 'about'}
                  onClick={() => setSettingsTab('about')}
                >
                  <span className="settings-tab-title">About</span>
                  <small>Version and updates</small>
                </button>
              </div>

              {settingsTab === 'general' && (
                <section className="settings-panel general-settings-panel">
                  <div className="settings-column settings-workspace">
                    <div className="settings-section-head">
                      <span className="panel-eyebrow">Core preferences</span>
                      <h2>General settings</h2>
                      <p>Keep the shell readable, then define how TuberSwitch responds when your avatar apps come and go.</p>
                    </div>
                    <div className="form-grid">
                      <label>
                        <FieldLabel text="Theme"/>
                        <select value={theme} onChange={(event) => setTheme(event.currentTarget.value as ThemeMode)}>
                          <option value="dark">Dark</option>
                          <option value="light">Light</option>
                        </select>
                      </label>
                      <label>
                        <FieldLabel text="Startup Mode"/>
                        <select value={draft.startupMode} onChange={(event) => setDraft({...draft, startupMode: event.currentTarget.value as Config['startupMode']})}>
                          <option value="restore-last">Restore Last Mode</option>
                          <option value="always-3d">Always 3D</option>
                          <option value="always-png">Always PNG</option>
                        </select>
                      </label>
                      <section className="settings-subsection">
                        <div className="settings-subsection-head">
                          <span className="panel-eyebrow">Automation</span>
                          <h3>App Detection</h3>
                          <p>Let TuberSwitch switch modes automatically based on the apps you actually launch.</p>
                        </div>
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
                        invalid={appDetectionIntervalInvalid}
                        helpText="Use 2 seconds or more to avoid noisy mode churn."
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
                    <div className="button-row">
                      <button type="button" className="icon-only-button" onClick={saveSettings} disabled={!!busy} aria-label="Save general settings" title="Save general settings">
                        <SaveIcon/>
                      </button>
                    </div>
                  </div>
                </section>
              )}

              {settingsTab === 'about' && (
                <section className="settings-panel about-settings-panel">
                  <div className="settings-column settings-workspace">
                    <div className="settings-section-head">
                      <span className="panel-eyebrow">Application</span>
                      <h2>About TuberSwitch</h2>
                      <p>Version {currentVersion} · Starsong Tools utility</p>
                    </div>
                    <div className="button-row">
                      <button type="button" onClick={checkForUpdates} disabled={updateBusy}>
                        <ButtonLabel icon={<RefreshIcon/>}>{updateBusy ? 'Checking for Updates' : 'Check for Updates'}</ButtonLabel>
                      </button>
                      {updateInfo?.updateAvailable && (
                        <button type="button" className="highlight-button" onClick={() => BrowserOpenURL(updateInfo.releaseUrl)}>
                          <ButtonLabel icon={<LaunchIcon/>}>Open GitHub Releases</ButtonLabel>
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

              {settingsTab === 'connections' && (
                <section className="settings-panel connections-settings-panel">
                  <div className="settings-column settings-workspace">
                    <div className="settings-section-head">
                      <span className="panel-eyebrow">External services</span>
                      <h2>Connections</h2>
                      <p>Connect TuberSwitch to OBS and Twitch. Profile-specific scene and reward behavior lives in Profiles.</p>
                    </div>
                    <div className="connections-grid">
                      <section className="connection-group">
                        <div className="settings-subsection-head">
                          <span className="panel-eyebrow">OBS</span>
                          <h3>WebSocket</h3>
                          <p>Connect to OBS. Scene and source choices are edited per profile.</p>
                        </div>
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
                            invalid={obsHostInvalid}
                            helpText="Usually 127.0.0.1 for a local OBS setup."
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
                            invalid={obsPortInvalid}
                            helpText="Port must be a positive number."
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
                            helpText={draft.obs.passwordConfigured ? 'A password is already stored securely. Enter a new value only if OBS changed.' : undefined}
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
                      </section>

                      <section className="connection-group">
                        <div className="settings-subsection-head">
                          <span className="panel-eyebrow">Twitch</span>
                          <h3>Auth</h3>
                          <p>Authenticate once, then use Profiles to decide which rewards follow each stream setup.</p>
                        </div>
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
                            invalid={twitchClientIdInvalid}
                            helpText="Required before Twitch login or reward refresh can work."
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
                        </div>
                        <div className="channel-name">
                          Authenticated Channel: <strong>{draft.twitch.channelName || 'Not connected'}</strong>
                        </div>
                      </section>
                    </div>

                    <div className="button-row">
                      <button className="icon-only-button" onClick={saveSettings} disabled={!!busy} aria-label="Save connection settings" title="Save connection settings">
                        <SaveIcon/>
                      </button>
                      <button onClick={() => saveThen('Logging in', () => StartTwitchLogin() as unknown as Promise<ActionResult>)} disabled={!!busy}>
                        <ButtonLabel icon={<LaunchIcon/>}>Login with Twitch</ButtonLabel>
                      </button>
                      <button onClick={() => saveThen('Refreshing rewards', () => RefreshTwitchRewards() as unknown as Promise<ActionResult>)} disabled={!!busy}>
                        <ButtonLabel icon={<RefreshIcon/>}>Refresh Rewards</ButtonLabel>
                      </button>
                    </div>
                  </div>
                </section>
              )}

              {settingsTab === 'profiles' && (
                <section className="settings-panel profiles-settings-panel">
                  <div className="settings-column settings-workspace">
                    <div className="settings-section-head">
                      <span className="panel-eyebrow">Stream setups</span>
                      <h2>Profiles</h2>
                      <p>Create and maintain reusable stream setups. Profiles own presentation mode, OBS mappings, and reward behavior.</p>
                    </div>
                    <div className="form-grid">
                      <label>
                        <FieldLabel text="Profile"/>
                        <select
                          value={draft.activeProfileId || 'default'}
                          onChange={(event) => void selectProfile(event.currentTarget.value)}
                          disabled={!canInteract}
                          aria-label="Profile"
                        >
                          {recentProfiles.length > 0 && (
                            <optgroup label="Recently Used">
                              {recentProfiles.map((profile) => (
                                <option key={profile.id} value={profile.id}>{profile.name}</option>
                              ))}
                            </optgroup>
                          )}
                          <optgroup label="All Profiles">
                            {remainingProfiles.map((profile) => (
                              <option key={profile.id} value={profile.id}>{profile.name}</option>
                            ))}
                            {recentProfiles.map((profile) => (
                              <option key={`all-${profile.id}`} value={profile.id}>{profile.name}</option>
                            ))}
                          </optgroup>
                        </select>
                      </label>
                      <label>
                        <FieldLabel text="Profile Mode"/>
                        <select value={draft.currentMode} onChange={(event) => setDraft({...draft, currentMode: event.currentTarget.value as Mode})}>
                          <option value="PNG">PNG</option>
                          <option value="3D">3D</option>
                        </select>
                      </label>
                    </div>
                    <div className="button-row">
                      <button className="icon-only-button" onClick={saveCurrentProfile} disabled={!!busy} aria-label="Save profile" title="Save profile">
                        <SaveIcon/>
                      </button>
                      <button type="button" onClick={saveCurrentProfileAs} disabled={!canInteract}>
                        <ButtonLabel icon={<PlusIcon/>}>Save As</ButtonLabel>
                      </button>
                      <button type="button" className="secondary icon-only-button" onClick={deleteCurrentProfile} disabled={!canInteract || draft.activeProfileId === 'default'} aria-label="Delete profile" title="Delete profile">
                        <TrashIcon/>
                      </button>
                      <button onClick={async () => {
                        const result = await saveProfileThen('Syncing OBS', () => SyncOBS() as unknown as Promise<ActionResult>);
                        await loadInventory(result?.newStatus?.config?.sceneMappings?.[0]?.scene || '');
                      }} disabled={!!busy}>
                        <ButtonLabel icon={<RefreshIcon/>}>Sync Scenes & Sources</ButtonLabel>
                      </button>
                    </div>
                    {profileDirty && <span className="profile-dirty-note">Unsaved profile changes *</span>}

                    <CollapsibleSection title="Scene Mapping" open={scenesOpen} onToggle={setScenesOpen}>
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
                          <span>Desired Source</span>
                        </div>
                        {(draft.sceneMappings || []).length === 0 && (
                          <EmptyStateRow
                            title="No scenes loaded yet"
                            body="Sync OBS to load your scenes, then choose the source this profile should enable in each scene."
                          />
                        )}
                        {visibleSceneMappings.map(({mapping, index}) => {
                          const sources = inventory.sourcesByScene?.[mapping.scene] || [];
                          const desiredSource = draft.currentMode === '3D' ? mapping.vtuberSource : mapping.pngTuberSource;
                          const missingSource = mapping.enabled && !desiredSource;
                          return (
                            <div className="scene-mapping-row" key={mapping.scene || index}>
                              <input
                                type="checkbox"
                                checked={mapping.enabled}
                                onChange={(event) => updateSceneMapping(index, {enabled: event.currentTarget.checked})}
                              />
                              <strong className="scene-name">
                                {missingSource && <span className="warning-icon" title="Choose the source this profile should enable in this scene">!</span>}
                                <span>{mapping.scene}</span>
                              </strong>
                              <SourceSelect
                                label="Desired Source"
                                value={desiredSource}
                                sources={sources}
                                onChange={(name, id) => updateDesiredSceneSource(index, name, id)}
                              />
                            </div>
                          );
                        })}
                      </div>
                    </CollapsibleSection>

                    <CollapsibleSection title="Create Reward" open={createRewardOpen} onToggle={setCreateRewardOpen}>
                      <div className="create-reward">
                        <TextInput label="New Reward Name" value={newRewardTitle} onChange={setNewRewardTitle}/>
                        <NumberInput label="Cost" value={newRewardCost} onChange={setNewRewardCost}/>
                        <TextInput label="Prompt (optional)" value={newRewardPrompt} onChange={setNewRewardPrompt}/>
                        <button className="highlight-button" onClick={createReward} disabled={!!busy || !newRewardTitle.trim()}>
                          <ButtonLabel icon={<PlusIcon/>}>Create Reward</ButtonLabel>
                        </button>
                      </div>
                    </CollapsibleSection>

                    <CollapsibleSection title="Manageable Rewards" open={manageableRewardsOpen} onToggle={setManageableRewardsOpen}>
                      <div className="reward-table">
                        <div className="reward-head">
                          <span>Reward Name</span>
                          <span>Enabled</span>
                        </div>
                        {manageableRewards.length === 0 && (
                          <EmptyStateRow
                            title="No manageable rewards loaded"
                            body="Refresh rewards after Twitch login to load rewards created by this client ID."
                          />
                        )}
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
                        {unmanageableRewards.length === 0 && (
                          <EmptyStateRow
                            title="No read-only rewards loaded"
                            body="If Twitch returns rewards that this app did not create, they will appear here as read-only."
                          />
                        )}
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
      <ProfileActionDialog
        kind={profileDialog}
        profileName={activeProfile?.name || 'this profile'}
        nameValue={profileNameInput}
        error={profileDialogError}
        busy={!!busy}
        onNameChange={setProfileNameInput}
        onClose={closeProfileDialog}
        onSaveAs={() => void confirmSaveProfileAs()}
        onDelete={() => void confirmDeleteCurrentProfile()}
      />
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

function TextInput({label, value, onChange, type = 'text', info, placeholder, helpText, invalid}: {label: string; value: string; onChange: (value: string) => void; type?: string; info?: ReactNode; placeholder?: string; helpText?: string; invalid?: boolean}) {
  return (
    <label>
      <FieldLabel text={label} info={info}/>
      <input className={invalid ? 'field-control invalid' : 'field-control'} aria-invalid={invalid || undefined} type={type} value={value || ''} placeholder={placeholder} onChange={(event) => onChange(event.currentTarget.value)}/>
      {helpText && <span className="field-help">{helpText}</span>}
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
        <button type="button" className="process-action-button" onClick={onSelectRunningApp}>
          <ButtonLabel icon={<ListIcon/>}>Select Running App</ButtonLabel>
        </button>
        <button type="button" className="process-action-button" onClick={onBrowseExecutable}>
          <ButtonLabel icon={<FolderIcon/>}>Browse Executable</ButtonLabel>
        </button>
      </div>
    </div>
  );
}

function NumberInput({label, value, onChange, info, min, helpText, invalid}: {label: string; value: number; onChange: (value: number) => void; info?: ReactNode; min?: number; helpText?: string; invalid?: boolean}) {
  return (
    <label>
      <FieldLabel text={label} info={info}/>
      <input className={invalid ? 'field-control invalid' : 'field-control'} aria-invalid={invalid || undefined} type="number" min={min} value={value || 0} onChange={(event) => onChange(Number(event.currentTarget.value))}/>
      {helpText && <span className="field-help">{helpText}</span>}
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
    <select
      className={!value ? 'field-control invalid' : 'field-control'}
      aria-label={label}
      aria-invalid={!value || undefined}
      value={value || ''}
      onChange={(event) => {
        const source = sources.find((item) => item.name === event.currentTarget.value);
        onChange(event.currentTarget.value, source?.sceneItemId || 0);
      }}
    >
      <option value="">Select...</option>
      {values.map((source) => <option key={`${source.name}-${source.sceneItemId}`} value={source.name}>{source.name}</option>)}
    </select>
  );
}

function EmptyStateRow({title, body}: {title: string; body: string}) {
  return (
    <div className="empty-row">
      <strong>{title}</strong>
      <span>{body}</span>
    </div>
  );
}

function ButtonLabel({icon, children}: {icon: ReactNode; children: ReactNode}) {
  return (
    <span className="button-label">
      <span className="button-icon" aria-hidden="true">{icon}</span>
      <span>{children}</span>
    </span>
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

function ProfileActionDialog({
  kind,
  profileName,
  nameValue,
  error,
  busy,
  onNameChange,
  onClose,
  onSaveAs,
  onDelete,
}: {
  kind: ProfileDialog;
  profileName: string;
  nameValue: string;
  error: string;
  busy: boolean;
  onNameChange: (value: string) => void;
  onClose: () => void;
  onSaveAs: () => void;
  onDelete: () => void;
}) {
  if (!kind) return null;

  const title = kind === 'save-as'
    ? 'Save Profile As'
    : 'Delete Profile';

  return (
    <div className="dialog-backdrop" onClick={onClose}>
      <section
        className="profile-action-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="profile-action-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="profile-action-header">
          <h3 id="profile-action-title">{title}</h3>
          <button type="button" className="secondary icon-only-button" onClick={onClose} aria-label="Close dialog">
            <CloseIcon/>
          </button>
        </div>

        {kind === 'save-as' && (
          <form
            className="profile-action-body"
            onSubmit={(event) => {
              event.preventDefault();
              onSaveAs();
            }}
          >
            <label>
              <FieldLabel text="Profile Name"/>
              <input
                autoFocus
                type="text"
                value={nameValue}
                aria-invalid={!!error || undefined}
                onChange={(event) => onNameChange(event.currentTarget.value)}
              />
            </label>
            {error && <div className="profile-action-error">{error}</div>}
            <div className="button-row profile-action-buttons">
              <button type="submit" disabled={busy}>
                <ButtonLabel icon={<SaveIcon/>}>Save</ButtonLabel>
              </button>
              <button type="button" className="secondary" onClick={onClose}>
                <ButtonLabel icon={<CloseIcon/>}>Cancel</ButtonLabel>
              </button>
            </div>
          </form>
        )}

        {kind === 'delete' && (
          <div className="profile-action-body">
            <p>Delete <strong>{profileName}</strong>? This removes the saved profile, but does not change OBS or Twitch.</p>
            <div className="button-row profile-action-buttons">
              <button type="button" className="danger-button" onClick={onDelete} disabled={busy}>
                <ButtonLabel icon={<TrashIcon/>}>Delete</ButtonLabel>
              </button>
              <button type="button" className="secondary" onClick={onClose}>
                <ButtonLabel icon={<CloseIcon/>}>Cancel</ButtonLabel>
              </button>
            </div>
          </div>
        )}

      </section>
    </div>
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
          <button type="button" className="secondary" onClick={onClose}>
            <ButtonLabel icon={<CloseIcon/>}>Cancel</ButtonLabel>
          </button>
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
          <button type="button" className="secondary" onClick={() => void loadProcesses(options)} disabled={loading}>
            <ButtonLabel icon={<RefreshIcon/>}>Refresh</ButtonLabel>
          </button>
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
          <button type="button" onClick={() => selectedProcess && onSelect(selectedProcess)} disabled={!selectedProcess}>
            <ButtonLabel icon={<CheckIcon/>}>Select</ButtonLabel>
          </button>
          <button type="button" className="secondary" onClick={onClose}>
            <ButtonLabel icon={<CloseIcon/>}>Cancel</ButtonLabel>
          </button>
        </div>
      </section>
    </div>
  );
}

function iconPath(path: string) {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d={path} />
    </svg>
  );
}

function GearIcon() {
  return iconPath('M6.7 1.6h2.6l.4 1.7a4.9 4.9 0 0 1 1.1.6l1.6-.8 1.3 2.2-1.3 1.1c.1.4.1.8.1 1.2s0 .8-.1 1.2l1.3 1.1-1.3 2.2-1.6-.8c-.3.2-.7.4-1.1.6l-.4 1.7H6.7l-.4-1.7a4.9 4.9 0 0 1-1.1-.6l-1.6.8-1.3-2.2 1.3-1.1A5.7 5.7 0 0 1 3.5 8c0-.4 0-.8.1-1.2L2.3 5.7 3.6 3.5l1.6.8c.3-.2.7-.4 1.1-.6l.4-1.7ZM8 10.3A2.3 2.3 0 1 0 8 5.7a2.3 2.3 0 0 0 0 4.6Z');
}

function SaveIcon() {
  return iconPath('M2.5 2.5h8.8l2.2 2.2v8.8H2.5v-11Zm2 0v3h5v-3m-4 11V9.6h5v3.9');
}

function RefreshIcon() {
  return iconPath('M12.6 4.9A5.4 5.4 0 1 0 13.3 10M12.6 4.9V2.7m0 2.2H10.4');
}

function SwitchIcon() {
  return iconPath('M3 5h8.5m0 0L9.2 2.7M11.5 5 9.2 7.3M13 11H4.5m0 0 2.3-2.3M4.5 11l2.3 2.3');
}

function LaunchIcon() {
  return iconPath('M6.1 3h6.9v6.9M13 3 7 9m-2.5-4.5H3.5v8h8V11');
}

function PlusIcon() {
  return iconPath('M8 3v10M3 8h10');
}

function ListIcon() {
  return iconPath('M5.5 4h7M5.5 8h7M5.5 12h7M3 4h.01M3 8h.01M3 12h.01');
}

function FolderIcon() {
  return iconPath('M2.5 4.5h3l1.2 1.4h6.8v5.6a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1v-7a1 1 0 0 1 1-1Z');
}

function CloseIcon() {
  return iconPath('M4.5 4.5 11.5 11.5M11.5 4.5 4.5 11.5');
}

function CheckIcon() {
  return iconPath('M3.5 8.2 6.6 11 12.5 5');
}

function TrashIcon() {
  return iconPath('M3.5 4.5h9M6 4.5V3h4v1.5m-5.5 0 .5 8h6l.5-8M7 6.8v3.8M9 6.8v3.8');
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

function findActiveProfile(config: Config) {
  return (config.profiles || []).find((profile) => profile.id === config.activeProfileId) || null;
}

function profileMatchesDraft(profile: Profile, config: Config) {
  return profile.mode === config.currentMode
    && stableStringify(profile.sources || {}) === stableStringify(config.sources || {})
    && stableStringify(profile.sceneMappings || []) === stableStringify(config.sceneMappings || [])
    && stableStringify(profile.rewardMappings || []) === stableStringify(config.rewardMappings || []);
}

function orderProfiles(profiles: Profile[]) {
  return [...profiles].sort((left, right) => {
    const leftTime = Date.parse(left.lastUsed || '') || 0;
    const rightTime = Date.parse(right.lastUsed || '') || 0;
    if (leftTime !== rightTime) return rightTime - leftTime;
    if (left.id === 'default') return -1;
    if (right.id === 'default') return 1;
    return left.name.localeCompare(right.name);
  });
}

function stableStringify(value: unknown) {
  return JSON.stringify(value);
}

export default App;
