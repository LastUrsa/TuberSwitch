export namespace app {
	
	export class Status {
	    config: Settings;
	    currentMode: string;
	    currentModeLabel: string;
	    obsConnected: boolean;
	    twitchConnected: boolean;
	    lastAction: string;
	
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
	export class Settings {
	    obs: OBSSettings;
	    sources: config.SourcesConfig;
	    sceneMappings: config.SceneMapping[];
	    twitch: TwitchSettings;
	    modeProfiles: config.ModeProfile[];
	    startupMode: string;
	    currentMode: string;
	    refreshRewardsOnStartup: boolean;

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

}

export namespace config {
	
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
	export class RewardMapping {
	    rewardId: string;
	    rewardName: string;
	    is3DOnly: boolean;
	    manageable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RewardMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rewardId = source["rewardId"];
	        this.rewardName = source["rewardName"];
	        this.is3DOnly = source["is3DOnly"];
	        this.manageable = source["manageable"];
	    }
	}
	export class TwitchConfig {
	    clientId: string;
	    channelId: string;
	    channelName: string;
	    accessToken: string;
	    refreshToken: string;
	    tokenExpiry: string;
	
	    static createFrom(source: any = {}) {
	        return new TwitchConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.channelId = source["channelId"];
	        this.channelName = source["channelName"];
	        this.accessToken = source["accessToken"];
	        this.refreshToken = source["refreshToken"];
	        this.tokenExpiry = source["tokenExpiry"];
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
	export class OBSConfig {
	    host: string;
	    port: number;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new OBSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.password = source["password"];
	    }
	}
	export class Config {
	    obs: OBSConfig;
	    sources: SourcesConfig;
	    sceneMappings: SceneMapping[];
	    twitch: TwitchConfig;
	    rewardMappings: RewardMapping[];
	    modeProfiles: ModeProfile[];
	    startupMode: string;
	    currentMode: string;
	    refreshRewardsOnStartup: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.obs = this.convertValues(source["obs"], OBSConfig);
	        this.sources = this.convertValues(source["sources"], SourcesConfig);
	        this.sceneMappings = this.convertValues(source["sceneMappings"], SceneMapping);
	        this.twitch = this.convertValues(source["twitch"], TwitchConfig);
	        this.rewardMappings = this.convertValues(source["rewardMappings"], RewardMapping);
	        this.modeProfiles = this.convertValues(source["modeProfiles"], ModeProfile);
	        this.startupMode = source["startupMode"];
	        this.currentMode = source["currentMode"];
	        this.refreshRewardsOnStartup = source["refreshRewardsOnStartup"];
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
	
	
	
	
	

}

