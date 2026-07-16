export namespace config {
	
	export class Step {
	    kind: string;
	    name?: string;
	    params?: Record<string, any>;
	    duration?: number;
	    lifetime?: string;
	    command?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new Step(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.params = source["params"];
	        this.duration = source["duration"];
	        this.lifetime = source["lifetime"];
	        this.command = source["command"];
	        this.message = source["message"];
	    }
	}
	export class Binding {
	    steps: Step[];
	    script?: string;
	    cooldownSeconds?: number;
	
	    static createFrom(source: any = {}) {
	        return new Binding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.steps = this.convertValues(source["steps"], Step);
	        this.script = source["script"];
	        this.cooldownSeconds = source["cooldownSeconds"];
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
	export class Command {
	    trigger: string;
	    binding: Binding;
	
	    static createFrom(source: any = {}) {
	        return new Command(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trigger = source["trigger"];
	        this.binding = this.convertValues(source["binding"], Binding);
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
	export class HookBinding {
	    id: string;
	    hookName: string;
	    idFilter?: number;
	    binding: Binding;
	
	    static createFrom(source: any = {}) {
	        return new HookBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.hookName = source["hookName"];
	        this.idFilter = source["idFilter"];
	        this.binding = this.convertValues(source["binding"], Binding);
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
	export class Redeem {
	    rewardId: string;
	    rewardTitle: string;
	    binding: Binding;
	
	    static createFrom(source: any = {}) {
	        return new Redeem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rewardId = source["rewardId"];
	        this.rewardTitle = source["rewardTitle"];
	        this.binding = this.convertValues(source["binding"], Binding);
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

export namespace main {
	
	export class ActivityEvent {
	    source: string;
	    user: string;
	    trigger: string;
	    error?: string;
	    at: string;
	
	    static createFrom(source: any = {}) {
	        return new ActivityEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.user = source["user"];
	        this.trigger = source["trigger"];
	        this.error = source["error"];
	        this.at = source["at"];
	    }
	}
	export class ServerStatus {
	    running: boolean;
	    port: number;
	    connectedClients: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.port = source["port"];
	        this.connectedClients = source["connectedClients"];
	    }
	}
	export class TwitchStatus {
	    loggedIn: boolean;
	    login: string;
	    needsClientId: boolean;
	    canPostChat: boolean;
	    eventSubError?: string;
	
	    static createFrom(source: any = {}) {
	        return new TwitchStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loggedIn = source["loggedIn"];
	        this.login = source["login"];
	        this.needsClientId = source["needsClientId"];
	        this.canPostChat = source["canPostChat"];
	        this.eventSubError = source["eventSubError"];
	    }
	}

}

export namespace sail {
	
	export class ParamSpec {
	    name: string;
	    type: string;
	    required: boolean;
	    default?: any;
	    min?: number;
	    max?: number;
	
	    static createFrom(source: any = {}) {
	        return new ParamSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.required = source["required"];
	        this.default = source["default"];
	        this.min = source["min"];
	        this.max = source["max"];
	    }
	}
	export class ActionInfo {
	    name: string;
	    displayName: string;
	    timed: boolean;
	    defaultDuration: number;
	    stacking: string;
	    valence: string;
	    params: ParamSpec[];
	
	    static createFrom(source: any = {}) {
	        return new ActionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.timed = source["timed"];
	        this.defaultDuration = source["defaultDuration"];
	        this.stacking = source["stacking"];
	        this.valence = source["valence"];
	        this.params = this.convertValues(source["params"], ParamSpec);
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
	export class HookInfo {
	    name: string;
	    idFilter: boolean;
	    filterField?: string;
	
	    static createFrom(source: any = {}) {
	        return new HookInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.idFilter = source["idFilter"];
	        this.filterField = source["filterField"];
	    }
	}

}

export namespace twitchapi {
	
	export class CustomReward {
	    id: string;
	    title: string;
	    cost: number;
	    is_enabled: boolean;
	    is_user_input_required: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CustomReward(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.cost = source["cost"];
	        this.is_enabled = source["is_enabled"];
	        this.is_user_input_required = source["is_user_input_required"];
	    }
	}

}

