export namespace main {

	export class ConnectivityCheck {
	    name: string;
	    target: string;
	    ok: boolean;
	    durationMs: number;
	    error?: string;
	    httpStatus?: number;
	    note?: string;

	    static createFrom(source: any = {}) {
	        return new ConnectivityCheck(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.target = source["target"];
	        this.ok = source["ok"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	        this.httpStatus = source["httpStatus"];
	        this.note = source["note"];
	    }
	}
	export class ProxyCandidate {
	    name: string;
	    address: string;
	    proxyUrl: string;
	    protocol: string;
	    source: string;

	    static createFrom(source: any = {}) {
	        return new ProxyCandidate(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	        this.proxyUrl = source["proxyUrl"];
	        this.protocol = source["protocol"];
	        this.source = source["source"];
	    }
	}
	export class ProbeResult {
	    candidate: ProxyCandidate;
	    ok: boolean;
	    checks: ConnectivityCheck[];
	    durationMs: number;
	    error?: string;
	    suggestion?: string;

	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidate = this.convertValues(source["candidate"], ProxyCandidate);
	        this.ok = source["ok"];
	        this.checks = this.convertValues(source["checks"], ConnectivityCheck);
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	        this.suggestion = source["suggestion"];
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
	export class DiagnosisReport {
	    direct: ProbeResult;
	    system?: ProbeResult;
	    localCandidates: ProbeResult[];
	    manual?: ProbeResult;
	    recommended?: ProbeResult;
	    summary: string;

	    static createFrom(source: any = {}) {
	        return new DiagnosisReport(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.direct = this.convertValues(source["direct"], ProbeResult);
	        this.system = this.convertValues(source["system"], ProbeResult);
	        this.localCandidates = this.convertValues(source["localCandidates"], ProbeResult);
	        this.manual = this.convertValues(source["manual"], ProbeResult);
	        this.recommended = this.convertValues(source["recommended"], ProbeResult);
	        this.summary = source["summary"];
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
