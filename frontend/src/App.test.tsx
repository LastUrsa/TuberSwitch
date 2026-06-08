import {beforeEach, describe, expect, it, vi} from 'vitest';
import {render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';

function getToggleButton(label: string) {
  return screen.getAllByRole('button').find((button) => button.textContent?.includes(label)) as HTMLButtonElement;
}

function getSettingsTab(name: 'General' | 'Connections' | 'Profiles' | 'About') {
  return screen.getByRole('tab', {name: new RegExp(`^${name}`, 'i')});
}

const mockStatus = {
  config: {
    obs: {host: '127.0.0.1', port: 4455, allowRemote: false, passwordConfigured: false},
    sources: {scene: '', vtuberSource: '', vtuberItemId: 0, pngTuberSource: '', pngTuberItemId: 0},
    sceneMappings: [
      {scene: 'Main', enabled: true, vtuberSource: 'VTuber', vtuberItemId: 10, pngTuberSource: '', pngTuberItemId: 0},
      {scene: 'BRB', enabled: false, vtuberSource: '', vtuberItemId: 0, pngTuberSource: '', pngTuberItemId: 0},
    ],
    twitch: {clientId: 'client', channelId: 'channel', channelName: 'Streamer'},
    rewardMappings: [
      {rewardId: 'manageable', rewardName: 'Dance', is3DOnly: true, manageable: true},
      {rewardId: 'readonly', rewardName: 'Hydrate', is3DOnly: false, manageable: false},
    ],
    profiles: [
      {
        id: 'default',
        name: 'Default',
        mode: 'PNG',
        sources: {scene: '', vtuberSource: '', vtuberItemId: 0, pngTuberSource: '', pngTuberItemId: 0},
        sceneMappings: [
          {scene: 'Main', enabled: true, vtuberSource: 'VTuber', vtuberItemId: 10, pngTuberSource: '', pngTuberItemId: 0},
          {scene: 'BRB', enabled: false, vtuberSource: '', vtuberItemId: 0, pngTuberSource: '', pngTuberItemId: 0},
        ],
        rewardMappings: [
          {rewardId: 'manageable', rewardName: 'Dance', is3DOnly: true, manageable: true},
          {rewardId: 'readonly', rewardName: 'Hydrate', is3DOnly: false, manageable: false},
        ],
        lastUsed: '',
      },
    ],
    activeProfileId: 'default',
    modeProfiles: [],
    startupMode: 'restore-last',
    currentMode: 'PNG',
    refreshRewardsOnStartup: false,
    appDetection: {
      enabled: false,
      threeDProcessName: '',
      pngProcessName: '',
      intervalSeconds: 5,
      conflictBehavior: 'do-nothing',
      applyTwitchChanges: true,
      manualOverrideCooldownSeconds: 15,
    },
  },
  currentMode: 'PNG',
  currentModeLabel: 'PNGTuber Mode',
  obsConnected: true,
  twitchConnected: true,
  lastAction: 'Ready',
  appDetectionStatus: 'Disabled',
  appDetectionEnabled: false,
};

const mockRewards = [
  {id: 'manageable', title: 'Dance', enabled: true, is3DOnly: true, manageable: true},
  {id: 'readonly', title: 'Hydrate', enabled: true, is3DOnly: false, manageable: false},
];

const actionResult = (status = mockStatus) => ({
  ok: true,
  message: 'ok',
  warnings: [],
  errors: [],
  newStatus: status,
});

const actionError = (message: string, status = mockStatus) => ({
  ok: false,
  message,
  warnings: [],
  errors: [message],
  newStatus: status,
});

const api = vi.hoisted(() => ({
  ApplyMode: vi.fn(),
  BrowseExecutable: vi.fn(),
  CheckForUpdates: vi.fn(),
  CreateTwitchReward: vi.fn(),
  GetOBSInventory: vi.fn(),
  GetStatus: vi.fn(),
  GetTwitchRewards: vi.fn(),
  ListRunningProcesses: vi.fn(),
  RefreshTwitchRewards: vi.fn(),
  DeleteProfile: vi.fn(),
  SaveConfig: vi.fn(),
  SaveProfile: vi.fn(),
  SaveProfileAs: vi.fn(),
  SelectProfile: vi.fn(),
  SetReward3DOnly: vi.fn(),
  StartTwitchLogin: vi.fn(),
  SyncOBS: vi.fn(),
}));

vi.mock('../wailsjs/go/main/App', () => api);

beforeEach(() => {
  vi.clearAllMocks();
  (window as typeof window & {runtime: Record<string, unknown>}).runtime = {
    LogError: vi.fn(),
    LogInfo: vi.fn(),
    WindowSetMinSize: vi.fn(),
    WindowSetSize: vi.fn(),
  };
  api.GetStatus.mockResolvedValue(structuredClone(mockStatus));
  api.GetTwitchRewards.mockResolvedValue(structuredClone(mockRewards));
  api.CheckForUpdates.mockResolvedValue({
    currentVersion: '0.5.0',
    latestVersion: '0.5.0',
    updateAvailable: false,
    releaseUrl: 'https://github.com/LastUrsa/TuberSwitch/releases',
    message: "You're running the latest version.",
  });
  api.GetOBSInventory.mockResolvedValue({
    scenes: [{name: 'Main'}, {name: 'BRB'}],
    sources: [],
    sourcesByScene: {
      Main: [{name: 'VTuber', sceneItemId: 10}, {name: 'PNG', sceneItemId: 11}],
      BRB: [{name: 'PNG BRB', sceneItemId: 20}],
    },
  });
  api.SaveConfig.mockImplementation(async (input) => actionResult({...mockStatus, config: input.config}));
  api.ListRunningProcesses.mockResolvedValue([
    {processName: 'AvatarApp.exe', pid: 1234},
    {processName: 'chrome.exe', pid: 999},
    {processName: 'Bitwarden.exe', pid: 5000},
    {processName: 'BackgroundAvatarHelper.exe', pid: 6000},
  ]);
  api.BrowseExecutable.mockResolvedValue('AvatarApp.exe');
  api.DeleteProfile.mockResolvedValue(actionResult());
  api.SaveProfile.mockImplementation(async (input) => actionResult({...mockStatus, config: input.config}));
  api.SaveProfileAs.mockImplementation(async (_name, input) => actionResult({...mockStatus, config: input.config}));
  api.SelectProfile.mockResolvedValue(actionResult());
  api.SetReward3DOnly.mockResolvedValue(actionResult());
  api.CreateTwitchReward.mockResolvedValue(actionResult());
  api.ApplyMode.mockResolvedValue(actionResult({...mockStatus, currentMode: '3D', currentModeLabel: '3D VTuber Mode'}));
  api.SyncOBS.mockResolvedValue(actionResult());
  api.StartTwitchLogin.mockResolvedValue(actionResult({
    ...mockStatus,
    twitchConnected: false,
    lastAction: 'Device code requested',
  }));
  api.RefreshTwitchRewards.mockResolvedValue(actionResult());
});

describe('App', () => {
  it('switches modes through the backend binding and updates the current mode label', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: /Switch to 3D VTuber mode/i}));

    await waitFor(() => expect(api.ApplyMode).toHaveBeenCalledWith('3D'));
    expect(await screen.findByRole('button', {name: /Switch to PNGTuber mode/i})).toBeInTheDocument();
    expect(screen.queryByText('3D live now')).not.toBeInTheDocument();
  });

  it('can change profiles directly from the main screen after a manual mode switch', async () => {
    const statusWithProfiles = structuredClone(mockStatus);
    statusWithProfiles.config.profiles.push({
      id: 'gaming',
      name: 'Gaming Stream',
      mode: '3D',
      sources: {scene: '', vtuberSource: '', vtuberItemId: 0, pngTuberSource: '', pngTuberItemId: 0},
      sceneMappings: [],
      rewardMappings: [],
      lastUsed: '',
    });
    api.GetStatus.mockResolvedValue(statusWithProfiles);
    const switchedStatus = structuredClone(statusWithProfiles);
    switchedStatus.currentMode = '3D';
    switchedStatus.currentModeLabel = '3D VTuber Mode';
    switchedStatus.config.currentMode = '3D';
    api.ApplyMode.mockResolvedValue(actionResult(switchedStatus));

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: /Switch to 3D VTuber mode/i}));
    await waitFor(() => expect(api.ApplyMode).toHaveBeenCalledWith('3D'));
    await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Active profile'}), 'gaming');

    await waitFor(() => expect(api.SelectProfile).toHaveBeenCalledWith('gaming'));
    expect(screen.queryByRole('dialog', {name: 'Unsaved Profile Changes'})).not.toBeInTheDocument();
    expect(api.SaveProfile).not.toHaveBeenCalled();
  });

  it('checks for updates from the About tab and shows the releases action when an update is available', async () => {
    api.CheckForUpdates.mockResolvedValueOnce({
      currentVersion: '0.5.0',
      latestVersion: '0.5.1',
      updateAvailable: true,
      releaseUrl: 'https://github.com/LastUrsa/TuberSwitch/releases',
      message: 'Version 0.5.1 is available.',
    });

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('About'));
    await userEvent.click(screen.getByRole('button', {name: 'Check for Updates'}));

    await waitFor(() => expect(api.CheckForUpdates).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('Version 0.5.1 is available.')).toBeInTheDocument();
    expect(screen.getByText('Latest version: 0.5.1')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Open GitHub Releases'})).toBeInTheDocument();
  });

  it('saves app detection settings from the General tab', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(screen.getByLabelText('Enable App Detection'));
    await userEvent.type(screen.getByLabelText(/3D Mode Process/i), 'vseeface.exe');
    await userEvent.type(screen.getByLabelText(/PNG Mode Process/i), 'pngtuber.exe');
    await userEvent.clear(screen.getByLabelText(/Detection Interval \(seconds\)/i));
    await userEvent.type(screen.getByLabelText(/Detection Interval \(seconds\)/i), '7');
    await userEvent.selectOptions(screen.getByLabelText('Conflict Behavior'), 'prefer-3d');
    await userEvent.click(screen.getByRole('button', {name: 'Save general settings'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.appDetection.enabled).toBe(true);
    expect(savedConfig.config.appDetection.threeDProcessName).toBe('vseeface.exe');
    expect(savedConfig.config.appDetection.pngProcessName).toBe('pngtuber.exe');
    expect(savedConfig.config.appDetection.intervalSeconds).toBe(7);
    expect(savedConfig.config.appDetection.conflictBehavior).toBe('prefer-3d');
  });

  it('selects a running process for the 3D field and stores only the executable name', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(screen.getAllByRole('button', {name: 'Select Running App'})[0]);
    expect(await screen.findByText('Select 3D Mode Process')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Search'), 'avatar');
    const processSelect = screen.getByLabelText('Running App');
    expect(screen.getAllByRole('option', {name: /AvatarApp\.exe/i})).toHaveLength(1);
    await userEvent.selectOptions(processSelect, 'AvatarApp.exe:1234');
    await userEvent.click(screen.getByRole('button', {name: /^Select$/}));

    expect(screen.getByLabelText(/3D Mode Process/i)).toHaveValue('AvatarApp.exe');
  });

  it('browses for an executable and stores only the filename for the PNG field', async () => {
    api.BrowseExecutable.mockResolvedValueOnce('C:\\Program Files\\Example Avatar App\\AvatarApp.exe');

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(screen.getAllByRole('button', {name: 'Browse Executable'})[1]);

    await waitFor(() => expect(api.BrowseExecutable).toHaveBeenCalledTimes(1));
    expect(screen.getByLabelText(/PNG Mode Process/i)).toHaveValue('AvatarApp.exe');
  });

  it('can show common desktop apps when that filter is disabled', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(screen.getAllByRole('button', {name: 'Select Running App'})[0]);
    await screen.findByText('Select 3D Mode Process');
    await userEvent.click(screen.getByLabelText('Likely avatar apps only'));
    await userEvent.click(screen.getByLabelText('Hide common desktop apps'));

    expect(await screen.findByRole('option', {name: /chrome\.exe \(PID 999\)/i})).toBeInTheDocument();
  });

  it('refreshes the running app picker with updated system-process filters', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(screen.getAllByRole('button', {name: 'Select Running App'})[0]);
    await screen.findByText('Select 3D Mode Process');
    await userEvent.click(screen.getByLabelText('Hide system processes'));
    await userEvent.click(screen.getByRole('button', {name: 'Refresh'}));

    await waitFor(() => expect(api.ListRunningProcesses).toHaveBeenLastCalledWith(expect.objectContaining({
      hideSystemProcesses: false,
    })));
  });

  it('shows process picker errors without closing the dialog', async () => {
    api.ListRunningProcesses.mockRejectedValueOnce(new Error('process list failed'));

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(screen.getAllByRole('button', {name: 'Select Running App'})[0]);

    expect(await screen.findByText('Error: process list failed')).toBeInTheDocument();
    expect(screen.getByText('Select 3D Mode Process')).toBeInTheDocument();
  });

  it('can show helper and utility apps when that filter is disabled', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(screen.getAllByRole('button', {name: 'Select Running App'})[0]);
    await screen.findByText('Select 3D Mode Process');
    await userEvent.click(screen.getByLabelText('Likely avatar apps only'));
    await userEvent.click(screen.getByLabelText('Hide helpers and utilities'));

    expect(await screen.findByRole('option', {name: /bitwarden\.exe \(PID 5000\)/i})).toBeInTheDocument();
  });

  it('can show background processes when visible-window filtering is disabled', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(screen.getAllByRole('button', {name: 'Select Running App'})[0]);
    await screen.findByText('Select 3D Mode Process');
    await userEvent.click(screen.getByLabelText('Likely avatar apps only'));
    await userEvent.click(screen.getByLabelText('Show only visible app windows'));
    await userEvent.type(screen.getByLabelText('Search'), 'backgroundavatar');

    expect(await screen.findByRole('option', {name: /backgroundavatarhelper\.exe \(PID 6000\)/i})).toBeInTheDocument();
  });

  it('orders main profile choices with default first, then names alphabetically', async () => {
    const statusWithProfiles = structuredClone(mockStatus);
    statusWithProfiles.config.profiles.push(
      {
        id: 'zed',
        name: 'Zed Stream',
        mode: 'PNG',
        sources: {scene: '', vtuberSource: '', vtuberItemId: 0, pngTuberSource: '', pngTuberItemId: 0},
        sceneMappings: [],
        rewardMappings: [],
        lastUsed: '',
      },
      {
        id: 'alpha',
        name: 'Alpha Stream',
        mode: 'PNG',
        sources: {scene: '', vtuberSource: '', vtuberItemId: 0, pngTuberSource: '', pngTuberItemId: 0},
        sceneMappings: [],
        rewardMappings: [],
        lastUsed: '',
      },
    );
    api.GetStatus.mockResolvedValue(statusWithProfiles);

    render(<App/>);
    await screen.findByText('TuberSwitch');

    const options = within(screen.getByRole('combobox', {name: 'Active profile'})).getAllByRole('option');
    expect(options.map((option) => option.textContent)).toEqual(['Default', 'Alpha Stream', 'Zed Stream']);
  });

  it('keeps an in-progress settings draft when the background status refresh runs', async () => {
    const user = userEvent.setup();

    api.GetStatus.mockResolvedValueOnce(structuredClone(mockStatus));
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await user.click(screen.getByRole('button', {name: 'Open settings'}));
    await user.click(getSettingsTab('Connections'));
    const hostInput = screen.getByLabelText(/OBS WebSocket Host/i);
    await user.clear(hostInput);
    await user.type(hostInput, 'draft-host');

    api.GetStatus.mockResolvedValueOnce({
      ...structuredClone(mockStatus),
      config: {
        ...structuredClone(mockStatus).config,
        obs: {...structuredClone(mockStatus).config.obs, host: 'refreshed-host'},
      },
    });

    await new Promise((resolve) => window.setTimeout(resolve, 2100));
    await waitFor(() => expect(api.GetStatus).toHaveBeenCalledTimes(2));
    expect(screen.getByLabelText(/OBS WebSocket Host/i)).toHaveValue('draft-host');
  });

  it('shows an error when the update check fails', async () => {
    api.CheckForUpdates.mockRejectedValueOnce(new Error('network down'));

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('About'));
    await userEvent.click(screen.getByRole('button', {name: 'Check for Updates'}));

    expect(await screen.findByText('Update check failed: Error: network down')).toBeInTheDocument();
  });

  it('shows manageable rewards as selectable and unmanageable rewards as read-only', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));

    const manageable = getToggleButton('Manageable Rewards').parentElement as HTMLElement;
    expect(within(manageable).getByText('Dance')).toBeInTheDocument();
    expect(within(manageable).getByRole('checkbox')).toBeEnabled();

    await userEvent.click(getToggleButton('Unmanageable Rewards'));
    const unmanageable = getToggleButton('Unmanageable Rewards').parentElement as HTMLElement;
    expect(within(unmanageable).getByText('Hydrate')).toBeInTheDocument();
    expect(within(unmanageable).getByText('Read-only')).toBeInTheDocument();
    expect(within(unmanageable).queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('shows a warning icon for enabled scene mappings with missing source selections', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));

    const warning = screen.getByTitle('Choose the source this profile should enable in this scene');
    expect(warning).toHaveTextContent('!');
    expect(screen.getByText('Main')).toBeInTheDocument();
  });

  it('filters scene mappings to selected scenes only', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    expect(screen.getByText('BRB')).toBeInTheDocument();

    await userEvent.click(screen.getByLabelText('Show only selected scenes'));
    expect(screen.queryByText('BRB')).not.toBeInTheDocument();
    expect(screen.getByText('1 of 2 scenes shown')).toBeInTheDocument();
  });

  it('creates a Twitch reward through the backend binding', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    await userEvent.click(getToggleButton('Create Reward'));
    const createRewardSection = getToggleButton('Create Reward').parentElement as HTMLElement;
    await userEvent.type(within(createRewardSection).getByLabelText(/New Reward Name/i), 'Throw Tomato');
    await userEvent.clear(within(createRewardSection).getByLabelText(/^Cost$/i));
    await userEvent.type(within(createRewardSection).getByLabelText(/^Cost$/i), '500');
    await userEvent.type(within(createRewardSection).getByLabelText(/Prompt \(optional\)/i), 'Aim carefully');
    await userEvent.click(within(createRewardSection).getByRole('button', {name: /^Create Reward$/}));

    await waitFor(() => expect(api.CreateTwitchReward).toHaveBeenCalledWith('Throw Tomato', 500, 'Aim carefully'));
  });

  it('saves draft OBS settings through the Save action', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Connections'));
    const hostInput = screen.getByLabelText(/OBS WebSocket Host/i);
    await userEvent.clear(hostInput);
    await userEvent.type(hostInput, 'obs-machine');
    await userEvent.click(screen.getByRole('button', {name: 'Save connection settings'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.obs.host).toBe('obs-machine');
  });

  it('syncs OBS and reloads the source inventory after saving settings', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    await userEvent.click(screen.getByRole('button', {name: 'Sync Scenes & Sources'}));

    await waitFor(() => expect(api.SaveProfile).toHaveBeenCalled());
    await waitFor(() => expect(api.SyncOBS).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(api.GetOBSInventory).toHaveBeenLastCalledWith('Main'));
  });

  it('saves Twitch connection settings before starting login', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Connections'));
    const clientIdInput = screen.getByLabelText(/Twitch Client ID/i);
    await userEvent.clear(clientIdInput);
    await userEvent.type(clientIdInput, 'new-client-id');
    await userEvent.click(screen.getByRole('button', {name: 'Login with Twitch'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.twitch.clientId).toBe('new-client-id');
    await waitFor(() => expect(api.StartTwitchLogin).toHaveBeenCalledTimes(1));
  });

  it('updates enabled reward selection through the backend binding', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    expect(screen.getByText('Enabled')).toBeInTheDocument();
    expect(screen.queryByText('3D Only')).not.toBeInTheDocument();
    const manageable = getToggleButton('Manageable Rewards').parentElement as HTMLElement;
    await userEvent.click(within(manageable).getByRole('checkbox'));

    await waitFor(() => expect(api.SetReward3DOnly).toHaveBeenCalledWith('manageable', false));
  });

  it('uses an in-app dialog when saving a profile as a new profile', async () => {
    const promptSpy = vi.spyOn(window, 'prompt').mockImplementation(() => 'Browser prompt should not open');

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    await userEvent.click(screen.getByRole('button', {name: 'Save As'}));

    expect(promptSpy).not.toHaveBeenCalled();
    const dialog = screen.getByRole('dialog', {name: 'Save Profile As'});
    await userEvent.type(within(dialog).getByLabelText('Profile Name'), 'Cozy Stream');
    await userEvent.click(within(dialog).getByRole('button', {name: 'Save'}));

    await waitFor(() => expect(api.SaveProfileAs).toHaveBeenCalledWith('Cozy Stream', expect.anything()));
    promptSpy.mockRestore();
  });

  it('shows backend validation errors returned from save operations', async () => {
    api.SaveConfig.mockResolvedValueOnce(actionError('OBS password is required'));

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Connections'));
    await userEvent.click(screen.getByRole('button', {name: 'Save connection settings'}));

    expect(await screen.findByText('OBS password is required')).toBeInTheDocument();
    expect(api.SyncOBS).not.toHaveBeenCalled();
  });

  it('persists startup mode changes when saving general settings', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.selectOptions(screen.getByLabelText('Startup Mode'), 'always-3d');
    await userEvent.click(screen.getByRole('button', {name: 'Save general settings'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.startupMode).toBe('always-3d');
  });

  it('refreshes Twitch rewards from the backend action', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Connections'));
    await userEvent.click(screen.getByRole('button', {name: 'Refresh Rewards'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    await waitFor(() => expect(api.RefreshTwitchRewards).toHaveBeenCalledTimes(1));
  });

  it('persists the refresh-on-startup setting when saving connection settings', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Connections'));
    await userEvent.click(screen.getByLabelText('Refresh Twitch rewards on startup'));
    await userEvent.click(screen.getByRole('button', {name: 'Save connection settings'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.refreshRewardsOnStartup).toBe(true);
  });

  it('saves the selected desired scene source with its OBS scene item id', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    await userEvent.click(screen.getByRole('button', {name: 'Sync Scenes & Sources'}));
    await waitFor(() => expect(api.GetOBSInventory).toHaveBeenLastCalledWith('Main'));
    expect(screen.queryByText('Selected a source')).not.toBeInTheDocument();
    const mainSceneRow = screen.getByText('Main').closest('.scene-mapping-row') as HTMLElement;
    await userEvent.selectOptions(within(mainSceneRow).getByRole('combobox', {name: 'Desired Source'}), 'PNG');
    await userEvent.click(screen.getByRole('button', {name: 'Save profile'}));

    await waitFor(() => expect(api.SaveProfile).toHaveBeenCalled());
    const savedConfig = api.SaveProfile.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.sceneMappings[0].pngTuberSource).toBe('PNG');
    expect(savedConfig.config.sceneMappings[0].pngTuberItemId).toBe(11);
  });

  it('toggles whether a scene is used by the active profile', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    const mainSceneRow = screen.getByText('Main').closest('.scene-mapping-row') as HTMLElement;
    await userEvent.click(within(mainSceneRow).getByRole('checkbox'));
    await userEvent.click(screen.getByRole('button', {name: 'Save profile'}));

    await waitFor(() => expect(api.SaveProfile).toHaveBeenCalled());
    const savedConfig = api.SaveProfile.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.sceneMappings[0].enabled).toBe(false);
  });

  it('enables a scene when a desired source is selected', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    await userEvent.click(screen.getByRole('button', {name: 'Sync Scenes & Sources'}));
    await waitFor(() => expect(api.GetOBSInventory).toHaveBeenLastCalledWith('Main'));
    const brbSceneRow = screen.getByText('BRB').closest('.scene-mapping-row') as HTMLElement;
    await userEvent.selectOptions(within(brbSceneRow).getByRole('combobox', {name: 'Desired Source'}), 'PNG BRB');
    await userEvent.click(screen.getByRole('button', {name: 'Save profile'}));

    await waitFor(() => expect(api.SaveProfile).toHaveBeenCalled());
    const savedConfig = api.SaveProfile.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.sceneMappings[1].enabled).toBe(true);
    expect(savedConfig.config.sceneMappings[1].pngTuberSource).toBe('PNG BRB');
    expect(savedConfig.config.sceneMappings[1].pngTuberItemId).toBe(20);
  });

  it('shows an empty state when a profile has no scene mappings', async () => {
    const statusWithoutScenes = structuredClone(mockStatus);
    statusWithoutScenes.config.sceneMappings = [];
    statusWithoutScenes.config.profiles[0].sceneMappings = [];
    api.GetStatus.mockResolvedValue(statusWithoutScenes);

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));

    expect(screen.getByText('No scenes loaded yet')).toBeInTheDocument();
  });

  it('deletes non-default profiles through the in-app dialog', async () => {
    const statusWithProfile = structuredClone(mockStatus);
    statusWithProfile.config.activeProfileId = 'gaming';
    statusWithProfile.config.profiles.push({
      id: 'gaming',
      name: 'Gaming Stream',
      mode: '3D',
      sources: {scene: '', vtuberSource: '', vtuberItemId: 0, pngTuberSource: '', pngTuberItemId: 0},
      sceneMappings: [],
      rewardMappings: [],
      lastUsed: '',
    });
    api.GetStatus.mockResolvedValue(statusWithProfile);

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    await userEvent.click(screen.getByRole('button', {name: 'Delete profile'}));
    const dialog = screen.getByRole('dialog', {name: 'Delete Profile'});
    await userEvent.click(within(dialog).getByRole('button', {name: 'Delete'}));

    await waitFor(() => expect(api.DeleteProfile).toHaveBeenCalledWith('gaming'));
  });

  it('validates OBS settings before syncing scenes and sources', async () => {
    api.SaveProfile.mockResolvedValueOnce(actionError('OBS password is required'));

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('Profiles'));
    await userEvent.click(screen.getByRole('button', {name: 'Sync Scenes & Sources'}));

    await waitFor(() => expect(api.SaveProfile).toHaveBeenCalled());
    expect(api.SyncOBS).not.toHaveBeenCalled();
  });

  it('hides the client secret field and config paths from the settings view', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    expect(screen.queryByText(/Config:/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Twitch Client Secret/i)).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    expect(screen.queryByText(/Config:/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Log:/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Twitch Client Secret/i)).not.toBeInTheDocument();
  });

  it('keeps the aligned settings chrome accessible through icon-only controls and the About panel', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Open settings'}));
    await userEvent.click(getSettingsTab('About'));

    expect(screen.getByRole('dialog', {name: 'Settings'})).toBeInTheDocument();
    expect(screen.getByRole('heading', {name: 'About TuberSwitch'})).toBeInTheDocument();
    expect(screen.getByText(/Starsong Tools utility/i)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Close settings'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Check for Updates'})).toBeInTheDocument();
  });
});
