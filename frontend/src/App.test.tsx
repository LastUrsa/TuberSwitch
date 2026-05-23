import {beforeEach, describe, expect, it, vi} from 'vitest';
import {render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';

function getToggleButton(label: string) {
  return screen.getAllByRole('button').find((button) => button.textContent?.includes(label)) as HTMLButtonElement;
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
    modeProfiles: [],
    startupMode: 'restore-last',
    currentMode: 'PNG',
    refreshRewardsOnStartup: false,
  },
  currentMode: 'PNG',
  currentModeLabel: 'PNG VTuber Mode',
  obsConnected: true,
  twitchConnected: true,
  lastAction: 'Ready',
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
  CreateTwitchReward: vi.fn(),
  GetOBSInventory: vi.fn(),
  GetStatus: vi.fn(),
  GetTwitchRewards: vi.fn(),
  RefreshTwitchRewards: vi.fn(),
  SaveConfig: vi.fn(),
  SetReward3DOnly: vi.fn(),
  StartTwitchLogin: vi.fn(),
  SyncOBS: vi.fn(),
  Test3DMode: vi.fn(),
  TestOBSConnection: vi.fn(),
  TestPNGMode: vi.fn(),
}));

vi.mock('../wailsjs/go/main/App', () => api);

beforeEach(() => {
  vi.clearAllMocks();
  api.GetStatus.mockResolvedValue(structuredClone(mockStatus));
  api.GetTwitchRewards.mockResolvedValue(structuredClone(mockRewards));
  api.GetOBSInventory.mockResolvedValue({
    scenes: [{name: 'Main'}, {name: 'BRB'}],
    sources: [],
    sourcesByScene: {
      Main: [{name: 'VTuber', sceneItemId: 10}, {name: 'PNG', sceneItemId: 11}],
      BRB: [{name: 'PNG BRB', sceneItemId: 20}],
    },
  });
  api.SaveConfig.mockImplementation(async (input) => actionResult({...mockStatus, config: input.config}));
  api.SetReward3DOnly.mockResolvedValue(actionResult());
  api.CreateTwitchReward.mockResolvedValue(actionResult());
  api.ApplyMode.mockResolvedValue(actionResult({...mockStatus, currentMode: '3D', currentModeLabel: '3D VTuber Mode'}));
  api.TestOBSConnection.mockResolvedValue(actionResult());
  api.SyncOBS.mockResolvedValue(actionResult());
  api.StartTwitchLogin.mockResolvedValue(actionResult({
    ...mockStatus,
    twitchConnected: false,
    lastAction: 'Device code requested',
  }));
  api.RefreshTwitchRewards.mockResolvedValue(actionResult());
  api.Test3DMode.mockResolvedValue(actionResult());
  api.TestPNGMode.mockResolvedValue(actionResult());
});

describe('App', () => {
  it('switches modes through the backend binding and updates the current mode label', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: /PNG VTuber Mode/i}));

    await waitFor(() => expect(api.ApplyMode).toHaveBeenCalledWith('3D'));
    expect(await screen.findByRole('button', {name: /3D VTuber Mode/i})).toBeInTheDocument();
    expect(screen.getByText('Current Mode').nextElementSibling).toHaveTextContent('3D VTuber Mode');
  });

  it('shows manageable rewards as selectable and unmanageable rewards as read-only', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));

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

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));

    const warning = screen.getByTitle('Only selected sources in this scene will be toggled');
    expect(warning).toHaveTextContent('!');
    expect(screen.getByText('Main')).toBeInTheDocument();
  });

  it('filters scene mappings to selected scenes only', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    expect(screen.getByText('BRB')).toBeInTheDocument();

    await userEvent.click(screen.getByLabelText('Show only selected scenes'));
    expect(screen.queryByText('BRB')).not.toBeInTheDocument();
    expect(screen.getByText('1 of 2 scenes shown')).toBeInTheDocument();
  });

  it('creates a Twitch reward through the backend binding', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    await userEvent.click(getToggleButton('Create Reward'));
    const createRewardSection = getToggleButton('Create Reward').parentElement as HTMLElement;
    await userEvent.type(within(createRewardSection).getByLabelText(/New Reward Name/i), 'Throw Tomato');
    await userEvent.clear(within(createRewardSection).getByLabelText(/^Cost$/i));
    await userEvent.type(within(createRewardSection).getByLabelText(/^Cost$/i), '500');
    await userEvent.type(within(createRewardSection).getByLabelText(/Prompt \(optional\)/i), 'Aim carefully');
    await userEvent.click(within(createRewardSection).getByRole('button', {name: /^Create Reward$/}));

    await waitFor(() => expect(api.CreateTwitchReward).toHaveBeenCalledWith('Throw Tomato', 500, 'Aim carefully'));
  });

  it('saves draft OBS settings before testing the OBS connection', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    const hostInput = screen.getByLabelText(/OBS WebSocket Host/i);
    await userEvent.clear(hostInput);
    await userEvent.type(hostInput, 'obs-machine');
    await userEvent.click(screen.getByRole('button', {name: 'Test OBS'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.obs.host).toBe('obs-machine');
    await waitFor(() => expect(api.TestOBSConnection).toHaveBeenCalledTimes(1));
  });

  it('syncs OBS and reloads the source inventory after saving settings', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    await userEvent.click(screen.getByRole('button', {name: 'Sync Scenes & Sources'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    await waitFor(() => expect(api.SyncOBS).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(api.GetOBSInventory).toHaveBeenLastCalledWith('Main'));
  });

  it('saves Twitch settings before starting login', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    const clientIdInput = screen.getByLabelText(/Twitch Client ID/i);
    await userEvent.clear(clientIdInput);
    await userEvent.type(clientIdInput, 'new-client-id');
    await userEvent.click(screen.getByRole('button', {name: 'Login with Twitch'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.twitch.clientId).toBe('new-client-id');
    await waitFor(() => expect(api.StartTwitchLogin).toHaveBeenCalledTimes(1));
  });

  it('updates 3D-only reward selection through the backend binding', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    const manageable = getToggleButton('Manageable Rewards').parentElement as HTMLElement;
    await userEvent.click(within(manageable).getByRole('checkbox'));

    await waitFor(() => expect(api.SetReward3DOnly).toHaveBeenCalledWith('manageable', false));
  });

  it('shows backend validation errors returned from save operations', async () => {
    api.SaveConfig.mockResolvedValueOnce(actionError('OBS password is required'));

    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    await userEvent.click(screen.getAllByRole('button', {name: 'Save'})[0]);

    expect(await screen.findByText('OBS password is required')).toBeInTheDocument();
    expect(api.TestOBSConnection).not.toHaveBeenCalled();
  });

  it('persists startup mode changes when saving Twitch settings', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    await userEvent.selectOptions(screen.getByLabelText('Startup Mode'), 'always-3d');
    await userEvent.click(screen.getAllByRole('button', {name: 'Save'})[1]);

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.startupMode).toBe('always-3d');
  });

  it('refreshes Twitch rewards from the backend action', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    await userEvent.click(screen.getByRole('button', {name: 'Refresh Rewards'}));

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    await waitFor(() => expect(api.RefreshTwitchRewards).toHaveBeenCalledTimes(1));
  });

  it('persists the refresh-on-startup setting when saving Twitch settings', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    await userEvent.click(screen.getByLabelText('Refresh Twitch rewards on startup'));
    await userEvent.click(screen.getAllByRole('button', {name: 'Save'})[1]);

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.refreshRewardsOnStartup).toBe(true);
  });

  it('saves selected scene sources with their OBS scene item ids', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    await userEvent.click(screen.getByRole('button', {name: 'Sync Scenes & Sources'}));
    await waitFor(() => expect(api.GetOBSInventory).toHaveBeenLastCalledWith('Main'));
    const pngSourceSelects = screen.getAllByLabelText('PNG Tuber Source');
    await userEvent.selectOptions(pngSourceSelects[0], 'PNG');
    await userEvent.click(screen.getAllByRole('button', {name: 'Save'})[0]);

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.sceneMappings[0].pngTuberSource).toBe('PNG');
    expect(savedConfig.config.sceneMappings[0].pngTuberItemId).toBe(11);
  });

  it('enables a scene when a VTuber source is selected', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    await userEvent.click(screen.getByRole('button', {name: 'Sync Scenes & Sources'}));
    await waitFor(() => expect(api.GetOBSInventory).toHaveBeenLastCalledWith('Main'));
    const vtuberSourceSelects = screen.getAllByLabelText('VTuber Source');
    await userEvent.selectOptions(vtuberSourceSelects[1], 'PNG BRB');
    await userEvent.click(screen.getAllByRole('button', {name: 'Save'})[0]);

    await waitFor(() => expect(api.SaveConfig).toHaveBeenCalled());
    const savedConfig = api.SaveConfig.mock.calls.at(-1)?.[0];
    expect(savedConfig.config.sceneMappings[1].enabled).toBe(true);
    expect(savedConfig.config.sceneMappings[1].vtuberSource).toBe('PNG BRB');
    expect(savedConfig.config.sceneMappings[1].vtuberItemId).toBe(20);
  });

  it('runs OBS-only test mode actions after saving settings', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    await userEvent.click(screen.getByRole('button', {name: 'Test 3D'}));
    await waitFor(() => expect(api.Test3DMode).toHaveBeenCalledTimes(1));

    await userEvent.click(screen.getByRole('button', {name: 'Test PNG'}));
    await waitFor(() => expect(api.TestPNGMode).toHaveBeenCalledTimes(1));
    expect(api.SaveConfig).toHaveBeenCalled();
  });

  it('hides the client secret field and config paths from the settings view', async () => {
    render(<App/>);
    await screen.findByText('TuberSwitch');

    expect(screen.queryByText(/Config:/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Twitch Client Secret/i)).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Settings'}));
    expect(screen.queryByText(/Config:/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Log:/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Twitch Client Secret/i)).not.toBeInTheDocument();
  });
});
