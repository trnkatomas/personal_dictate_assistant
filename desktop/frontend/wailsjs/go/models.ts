export namespace main {
	
	export class Settings {
	    mode: string;
	    whisperUrl: string;
	    whisperLanguage: string;
	    whisperTask: string;
	    modelName: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.whisperUrl = source["whisperUrl"];
	        this.whisperLanguage = source["whisperLanguage"];
	        this.whisperTask = source["whisperTask"];
	        this.modelName = source["modelName"];
	    }
	}
	export class SetupState {
	    binaryExists: boolean;
	    modelExists: boolean;
	    modelName: string;
	    binaryPath: string;
	    modelPath: string;
	
	    static createFrom(source: any = {}) {
	        return new SetupState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.binaryExists = source["binaryExists"];
	        this.modelExists = source["modelExists"];
	        this.modelName = source["modelName"];
	        this.binaryPath = source["binaryPath"];
	        this.modelPath = source["modelPath"];
	    }
	}
	export class SystemInfo {
	    gpu: string;
	    ramGB: number;
	    locale: string;
	    platform: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gpu = source["gpu"];
	        this.ramGB = source["ramGB"];
	        this.locale = source["locale"];
	        this.platform = source["platform"];
	    }
	}

}

