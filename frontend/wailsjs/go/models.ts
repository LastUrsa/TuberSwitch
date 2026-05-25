export namespace app {
	
	export class TwitchSettings {
	    clientId: string;
	    channelId: string;
	    channelName: string;
	
	    static createFrom(source: any = {}) {
	        return new TwitchSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.channelId = source["channelId"];
	        this.channelName = source["channelName"];
	    }
	}
	export class OBSSettings {
	    host: string;
	    port: number;
	    allowRemote: boolean;
	    passwordConfigured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OBSSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.allowRemote = source["allowRemote"];
	        this.passwordConfigured = source["passwordConfigured"];
	    }
	}
	export class Settings {
	    obs: OBSSettings;
	    sources: config.SourcesConfig;
	    sceneMappings: config.SceneMapping[];
	    twitch: TwitchSettings;
	    modeProfiles: config.ModeProfile[];
	    startupMode: string;
	    currentMode: string;
	    refreshRewardsOnStartup: boolean;
	    appDetection: config.AppDetectionConfig;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.obs = this.convertValues(source["obs"], OBSSettings);
	        this.sources = this.convertValues(source["sources"], config.SourcesConfig);
	        this.sceneMappings = this.convertValues(source["sceneMappings"], config.SceneMapping);
	        this.twitch = this.convertValues(source["twitch"], TwitchSettings);
	        this.modeProfiles = this.convertValues(source["modeProfiles"], config.ModeProfile);
	        this.startupMode = source["startupMode"];
	        this.currentMode = source["currentMode"];
	        this.refreshRewardsOnStartup = source["refreshRewardsOnStartup"];
	        this.appDetection = this.convertValues(source["appDetection"], config.AppDetectionConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Status {
	    config: Settings;
	    currentMode: string;
	    currentModeLabel: string;
	    obsConnected: boolean;
	    twitchConnected: boolean;
	    lastAction: string;
	    appDetectionStatus: string;
	    appDetectionEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], Settings);
	        this.currentMode = source["currentMode"];
	        this.currentModeLabel = source["currentModeLabel"];
	        this.obsConnected = source["obsConnected"];
	        this.twitchConnected = source["twitchConnected"];
	        this.lastAction = source["lastAction"];
	        this.appDetectionStatus = source["appDetectionStatus"];
	        this.appDetectionEnabled = source["appDetectionEnabled"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ActionResult {
	    ok: boolean;
	    message: string;
	    warnings: string[];
	    errors: string[];
	    newStatus: Status;
	
	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.warnings = source["warnings"];
	        this.errors = source["errors"];
	        this.newStatus = this.convertValues(source["newStatus"], Status);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OBSSource {
	    name: string;
	    sceneItemId: number;
	
	    static createFrom(source: any = {}) {
	        return new OBSSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.sceneItemId = source["sceneItemId"];
	    }
	}
	export class OBSScene {
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new OBSScene(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	    }
	}
	export class OBSInventory {
	    scenes: OBSScene[];
	    sources: OBSSource[];
	    sourcesByScene: Record<string, Array<OBSSource>>;
	
	    static createFrom(source: any = {}) {
	        return new OBSInventory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scenes = this.convertValues(source["scenes"], OBSScene);
	        this.sources = this.convertValues(source["sources"], OBSSource);
	        this.sourcesByScene = this.convertValues(source["sourcesByScene"], Array<OBSSource>, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class ProcessListOptions {
	    search: string;
	    showOnlyVisibleApps: boolean;
	    hideSystemProcesses: boolean;
	    hideCommonDesktopApps: boolean;
	    hideHelpersAndUtilities: boolean;
	    likelyAvatarAppsOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProcessListOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.search = source["search"];
	        this.showOnlyVisibleApps = source["showOnlyVisibleApps"];
	        this.hideSystemProcesses = source["hideSystemProcesses"];
	        this.hideCommonDesktopApps = source["hideCommonDesktopApps"];
	        this.hideHelpersAndUtilities = source["hideHelpersAndUtilities"];
	        this.likelyAvatarAppsOnly = source["likelyAvatarAppsOnly"];
	    }
	}
	export class ProcessSummary {
	    processName: string;
	    pid: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.processName = source["processName"];
	        this.pid = source["pid"];
	    }
	}
	
	export class SettingsInput {
	    config: Settings;
	    obsPassword: string;
	    updateObsPassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], Settings);
	        this.obsPassword = source["obsPassword"];
	        this.updateObsPassword = source["updateObsPassword"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class TwitchReward {
	    id: string;
	    title: string;
	    enabled: boolean;
	    is3DOnly: boolean;
	    manageable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TwitchReward(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.enabled = source["enabled"];
	        this.is3DOnly = source["is3DOnly"];
	        this.manageable = source["manageable"];
	    }
	}
	
	export class UpdateInfo {
	    currentVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	    releaseUrl: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.releaseUrl = source["releaseUrl"];
	        this.message = source["message"];
	    }
	}

}

export namespace config {
	
	export class AppDetectionConfig {
	    enabled: boolean;
	    threeDProcessName: string;
	    pngProcessName: string;
	    intervalSeconds: number;
	    conflictBehavior: string;
	    applyTwitchChanges: boolean;
	    manualOverrideCooldownSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new AppDetectionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.threeDProcessName = source["threeDProcessName"];
	        this.pngProcessName = source["pngProcessName"];
	        this.intervalSeconds = source["intervalSeconds"];
	        this.conflictBehavior = source["conflictBehavior"];
	        this.applyTwitchChanges = source["applyTwitchChanges"];
	        this.manualOverrideCooldownSeconds = source["manualOverrideCooldownSeconds"];
	    }
	}
	export class ModeProfile {
	    id: string;
	    displayName: string;
	    vtuberVisible: boolean;
	    pngTuberVisible: boolean;
	    enable3DRewards: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModeProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.vtuberVisible = source["vtuberVisible"];
	        this.pngTuberVisible = source["pngTuberVisible"];
	        this.enable3DRewards = source["enable3DRewards"];
	    }
	}
	export class SceneMapping {
	    scene: string;
	    enabled: boolean;
	    vtuberSource: string;
	    vtuberItemId: number;
	    pngTuberSource: string;
	    pngTuberItemId: number;
	
	    static createFrom(source: any = {}) {
	        return new SceneMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scene = source["scene"];
	        this.enabled = source["enabled"];
	        this.vtuberSource = source["vtuberSource"];
	        this.vtuberItemId = source["vtuberItemId"];
	        this.pngTuberSource = source["pngTuberSource"];
	        this.pngTuberItemId = source["pngTuberItemId"];
	    }
	}
	export class SourcesConfig {
	    scene: string;
	    vtuberSource: string;
	    vtuberItemId: number;
	    pngTuberSource: string;
	    pngTuberItemId: number;
	
	    static createFrom(source: any = {}) {
	        return new SourcesConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scene = source["scene"];
	        this.vtuberSource = source["vtuberSource"];
	        this.vtuberItemId = source["vtuberItemId"];
	        this.pngTuberSource = source["pngTuberSource"];
	        this.pngTuberItemId = source["pngTuberItemId"];
	    }
	}

}

