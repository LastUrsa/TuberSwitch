import {type ReactNode, useEffect, useMemo, useState} from 'react';
import './App.css';
import {
  ApplyMode,
  CreateTwitchReward,
  GetOBSInventory,
  GetStatus,
  GetTwitchRewards,
  RefreshTwitchRewards,
  SaveConfig,
  SetReward3DOnly,
  StartTwitchLogin,
  SyncOBS,
  Test3DMode,
  TestOBSConnection,
  TestPNGMode,
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

type TwitchReward = {
  id: string;
  title: string;
  enabled: boolean;
  is3DOnly: boolean;
  manageable: boolean;
};

const emptyInventory: OBSInventory = {scenes: [], sources: [], sourcesByScene: {}};

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

  useEffect(() => {
    load();
  }, []);

  async function load() {
    const next = await GetStatus();
    setStatus(next as unknown as Status);
    setDraft(structuredClone((next as unknown as Status).config));
    setRewards((await GetTwitchRewards()) as TwitchReward[]);
    setObsPassword('');
    setObsPasswordDirty(false);
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
        <div>
          <h1>TuberSwitch</h1>
          <p>{currentMode}</p>
        </div>
        <button className="secondary" onClick={() => setSettingsOpen(!settingsOpen)}>
          Settings
        </button>
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

      <section className="status-grid">
        <StatusPill
          label="OBS Connection"
          value={status?.obsConnected ? 'Connected' : 'Disconnected'}
          good={!!status?.obsConnected}
          info={(
            <>
              <p>Enable OBS WebSocket in <strong>Tools &gt; WebSocket Server Settings</strong> before testing here.</p>
            </>
          )}
        />
        <StatusPill
          label="Twitch Connection"
          value={status?.twitchConnected ? 'Connected' : 'Disconnected'}
          good={!!status?.twitchConnected}
          info={(
            <>
              <p>Create a Twitch app in the <a href="https://dev.twitch.tv/console/apps" target="_blank" rel="noreferrer">Developer Console</a>.</p>
              <p>Recommended setup: public app, scopes for <code>channel:read:redemptions</code> and <code>channel:manage:redemptions</code>, redirect <code>https://localhost</code> if Twitch requires one.</p>
            </>
          )}
        />
        <div className="status-card wide">
          <span>Last Action</span>
          <strong>{busy || status?.lastAction || 'Ready'}</strong>
        </div>
      </section>

      {errors.length > 0 && (
        <section className="error-list">
          {errors.map((error) => <div key={error}>{error}</div>)}
        </section>
      )}

      {settingsOpen && draft && (
        <>
          <section className="settings-layout">
            <div className="settings-column obs-settings">
              <h2>OBS Settings</h2>
              <div className="form-grid">
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
                <label className="check-row">
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
                <button onClick={() => saveThen('Testing OBS', () => TestOBSConnection() as unknown as Promise<ActionResult>)} disabled={!!busy}>Test OBS</button>
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

              <div className="button-row">
                <button onClick={() => saveThen('Testing 3D OBS', () => Test3DMode() as unknown as Promise<ActionResult>)} disabled={!!busy}>Test 3D</button>
                <button onClick={() => saveThen('Testing PNG OBS', () => TestPNGMode() as unknown as Promise<ActionResult>)} disabled={!!busy}>Test PNG</button>
              </div>
            </div>

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
                  value={draft.twitch.clientId}
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

        </>
      )}
    </main>
  );
}

function StatusPill({label, value, good, info}: {label: string; value: string; good: boolean; info?: ReactNode}) {
  return (
    <div className={`status-card ${good ? 'good' : 'bad'}`}>
      <span className="label-row">
        {label}
        {info && <InfoTip>{info}</InfoTip>}
      </span>
      <strong>{value}</strong>
    </div>
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

function NumberInput({label, value, onChange, info}: {label: string; value: number; onChange: (value: number) => void; info?: ReactNode}) {
  return (
    <label>
      <FieldLabel text={label} info={info}/>
      <input type="number" value={value || 0} onChange={(event) => onChange(Number(event.currentTarget.value))}/>
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

function buildSettingsInput(config: Config, obsPassword: string, updateObsPassword: boolean): SettingsInput {
  return {
    config,
    obsPassword,
    updateObsPassword,
  };
}

export default App;
