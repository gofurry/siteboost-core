export namespace main {

	export class CredentialLoginStartRequest {
	    accountName: string;
	    password: string;

	    static createFrom(source: any = {}) {
	        return new CredentialLoginStartRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountName = source["accountName"];
	        this.password = source["password"];
	    }
	}
	export class Freebie {
	    appID: string;
	    packageID?: number;
	    packageTitle?: string;
	    title: string;
	    storeURL: string;
	    capsuleURL: string;
	    released: string;
	    originalPrice: string;
	    finalPrice: string;
	    discount: string;
	    source: string;
	    status: string;
	    note: string;
	    firstSeenAt: string;
	    lastSeenAt: string;
	    updatedAt: string;

	    static createFrom(source: any = {}) {
	        return new Freebie(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appID = source["appID"];
	        this.packageID = source["packageID"];
	        this.packageTitle = source["packageTitle"];
	        this.title = source["title"];
	        this.storeURL = source["storeURL"];
	        this.capsuleURL = source["capsuleURL"];
	        this.released = source["released"];
	        this.originalPrice = source["originalPrice"];
	        this.finalPrice = source["finalPrice"];
	        this.discount = source["discount"];
	        this.source = source["source"];
	        this.status = source["status"];
	        this.note = source["note"];
	        this.firstSeenAt = source["firstSeenAt"];
	        this.lastSeenAt = source["lastSeenAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class FreebieSnapshot {
	    items: Freebie[];
	    total: number;
	    todoCount: number;
	    claimedCount: number;
	    skippedCount: number;
	    failedCount: number;
	    lastRefreshAt: string;
	    sourceURL: string;

	    static createFrom(source: any = {}) {
	        return new FreebieSnapshot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Freebie);
	        this.total = source["total"];
	        this.todoCount = source["todoCount"];
	        this.claimedCount = source["claimedCount"];
	        this.skippedCount = source["skippedCount"];
	        this.failedCount = source["failedCount"];
	        this.lastRefreshAt = source["lastRefreshAt"];
	        this.sourceURL = source["sourceURL"];
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
	export class FreebieClaimResult {
	    ok: boolean;
	    appID: string;
	    packageID?: number;
	    message: string;
	    cookieDomains: number;
	    snapshot?: FreebieSnapshot;

	    static createFrom(source: any = {}) {
	        return new FreebieClaimResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.appID = source["appID"];
	        this.packageID = source["packageID"];
	        this.message = source["message"];
	        this.cookieDomains = source["cookieDomains"];
	        this.snapshot = this.convertValues(source["snapshot"], FreebieSnapshot);
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

	export class GuardAction {
	    type: string;
	    detail?: string;

	    static createFrom(source: any = {}) {
	        return new GuardAction(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.detail = source["detail"];
	    }
	}
	export class LoginStartResult {
	    loginId: string;
	    status: string;
	    pollIntervalSecs: number;
	    validActions?: GuardAction[];
	    expiresAt: string;
	    safeStatusMessage: string;

	    static createFrom(source: any = {}) {
	        return new LoginStartResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loginId = source["loginId"];
	        this.status = source["status"];
	        this.pollIntervalSecs = source["pollIntervalSecs"];
	        this.validActions = this.convertValues(source["validActions"], GuardAction);
	        this.expiresAt = source["expiresAt"];
	        this.safeStatusMessage = source["safeStatusMessage"];
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
	export class LoginStatus {
	    loginId: string;
	    status: string;
	    steamId?: string;
	    account?: string;
	    pollIntervalSecs: number;
	    validActions?: GuardAction[];
	    expiresAt?: string;
	    safeStatusMessage: string;

	    static createFrom(source: any = {}) {
	        return new LoginStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loginId = source["loginId"];
	        this.status = source["status"];
	        this.steamId = source["steamId"];
	        this.account = source["account"];
	        this.pollIntervalSecs = source["pollIntervalSecs"];
	        this.validActions = this.convertValues(source["validActions"], GuardAction);
	        this.expiresAt = source["expiresAt"];
	        this.safeStatusMessage = source["safeStatusMessage"];
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
	export class NetworkConfig {
	    proxyUrl: string;

	    static createFrom(source: any = {}) {
	        return new NetworkConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyUrl = source["proxyUrl"];
	    }
	}
	export class NetworkConfigRequest {
	    proxyUrl: string;

	    static createFrom(source: any = {}) {
	        return new NetworkConfigRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyUrl = source["proxyUrl"];
	    }
	}
	export class QRLoginStartResult {
	    loginId: string;
	    qrChallengeUrl: string;
	    status: string;
	    pollIntervalSecs: number;
	    validActions?: GuardAction[];
	    expiresAt: string;
	    safeStatusMessage: string;

	    static createFrom(source: any = {}) {
	        return new QRLoginStartResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loginId = source["loginId"];
	        this.qrChallengeUrl = source["qrChallengeUrl"];
	        this.status = source["status"];
	        this.pollIntervalSecs = source["pollIntervalSecs"];
	        this.validActions = this.convertValues(source["validActions"], GuardAction);
	        this.expiresAt = source["expiresAt"];
	        this.safeStatusMessage = source["safeStatusMessage"];
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
	export class SteamAccountSummary {
	    steamId: string;
	    account: string;
	    loggedIn: boolean;
	    lastLoginAt?: string;

	    static createFrom(source: any = {}) {
	        return new SteamAccountSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.steamId = source["steamId"];
	        this.account = source["account"];
	        this.loggedIn = source["loggedIn"];
	        this.lastLoginAt = source["lastLoginAt"];
	    }
	}
	export class SubmitGuardCodeRequest {
	    loginId: string;
	    code: string;
	    type: string;

	    static createFrom(source: any = {}) {
	        return new SubmitGuardCodeRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loginId = source["loginId"];
	        this.code = source["code"];
	        this.type = source["type"];
	    }
	}
	export class WebSessionTestResult {
	    ok: boolean;
	    steamId?: string;
	    account?: string;
	    cookieDomains: number;
	    communityOk: boolean;
	    storeOk: boolean;
	    lastCookieRefreshAt?: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new WebSessionTestResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.steamId = source["steamId"];
	        this.account = source["account"];
	        this.cookieDomains = source["cookieDomains"];
	        this.communityOk = source["communityOk"];
	        this.storeOk = source["storeOk"];
	        this.lastCookieRefreshAt = source["lastCookieRefreshAt"];
	        this.message = source["message"];
	    }
	}

}
