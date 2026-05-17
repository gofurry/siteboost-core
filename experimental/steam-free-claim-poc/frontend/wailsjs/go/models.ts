export namespace main {
	
	export class Freebie {
	    appID: string;
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

}

