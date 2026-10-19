export namespace dto {
	
	export class Tag {
	    id: string;
	    kind: string;
	    name: string;
	    aliases: string[];
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new Tag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.aliases = source["aliases"];
	        this.count = source["count"];
	    }
	}
	export class NewTag {
	    key: string;
	    kind: string;
	    name: string;
	    aliases: string[];
	
	    static createFrom(source: any = {}) {
	        return new NewTag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.aliases = source["aliases"];
	    }
	}
	export class Asset {
	    id: string;
	    revision: number;
	    name: string;
	    boothId: string;
	    categoryId: string;
	    coverId: string;
	    coverToken: string;
	    universal: boolean;
	    notes: string;
	    folderPath: string;
	    allowOfflineFolder: boolean;
	    avatarIds: string[];
	    freeTagIds: string[];
	    newTags: NewTag[];
	    avatars: Tag[];
	    freeTags: Tag[];
	    createdAt: string;
	    updatedAt: string;
	    coverUrl: string;
	    thumbnailUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Asset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.revision = source["revision"];
	        this.name = source["name"];
	        this.boothId = source["boothId"];
	        this.categoryId = source["categoryId"];
	        this.coverId = source["coverId"];
	        this.coverToken = source["coverToken"];
	        this.universal = source["universal"];
	        this.notes = source["notes"];
	        this.folderPath = source["folderPath"];
	        this.allowOfflineFolder = source["allowOfflineFolder"];
	        this.avatarIds = source["avatarIds"];
	        this.freeTagIds = source["freeTagIds"];
	        this.newTags = this.convertValues(source["newTags"], NewTag);
	        this.avatars = this.convertValues(source["avatars"], Tag);
	        this.freeTags = this.convertValues(source["freeTags"], Tag);
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.coverUrl = source["coverUrl"];
	        this.thumbnailUrl = source["thumbnailUrl"];
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
	export class AssetDraft {
	    id: string;
	    revision: number;
	    name: string;
	    boothId: string;
	    categoryId: string;
	    coverId: string;
	    coverToken: string;
	    universal: boolean;
	    notes: string;
	    folderPath: string;
	    allowOfflineFolder: boolean;
	    avatarIds: string[];
	    freeTagIds: string[];
	    newTags: NewTag[];
	
	    static createFrom(source: any = {}) {
	        return new AssetDraft(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.revision = source["revision"];
	        this.name = source["name"];
	        this.boothId = source["boothId"];
	        this.categoryId = source["categoryId"];
	        this.coverId = source["coverId"];
	        this.coverToken = source["coverToken"];
	        this.universal = source["universal"];
	        this.notes = source["notes"];
	        this.folderPath = source["folderPath"];
	        this.allowOfflineFolder = source["allowOfflineFolder"];
	        this.avatarIds = source["avatarIds"];
	        this.freeTagIds = source["freeTagIds"];
	        this.newTags = this.convertValues(source["newTags"], NewTag);
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
	export class AssetPage {
	    items: Asset[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new AssetPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Asset);
	        this.total = source["total"];
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
	export class AssetQuery {
	    keyword: string;
	    categoryIds: string[];
	    uncategorized: boolean;
	    avatarIds: string[];
	    avatarMode: string;
	    avatarMatch: string;
	    freeTagIds: string[];
	    freeMode: string;
	    freeMatch: string;
	    folder: string;
	    sort: string;
	    descending: boolean;
	    offset: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new AssetQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keyword = source["keyword"];
	        this.categoryIds = source["categoryIds"];
	        this.uncategorized = source["uncategorized"];
	        this.avatarIds = source["avatarIds"];
	        this.avatarMode = source["avatarMode"];
	        this.avatarMatch = source["avatarMatch"];
	        this.freeTagIds = source["freeTagIds"];
	        this.freeMode = source["freeMode"];
	        this.freeMatch = source["freeMatch"];
	        this.folder = source["folder"];
	        this.sort = source["sort"];
	        this.descending = source["descending"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	    }
	}
	export class BackupInfo {
	    inspectionId: string;
	    createdAt: string;
	    formatVersion: string;
	    schemaVersion: number;
	    assetCount: number;
	    totalBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new BackupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inspectionId = source["inspectionId"];
	        this.createdAt = source["createdAt"];
	        this.formatVersion = source["formatVersion"];
	        this.schemaVersion = source["schemaVersion"];
	        this.assetCount = source["assetCount"];
	        this.totalBytes = source["totalBytes"];
	    }
	}
	export class BatchRequest {
	    ids: string[];
	    operation: string;
	    categoryId: string;
	    tagIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new BatchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ids = source["ids"];
	        this.operation = source["operation"];
	        this.categoryId = source["categoryId"];
	        this.tagIds = source["tagIds"];
	    }
	}
	export class Preferences {
	    language: string;
	    theme: string;
	    view: string;
	    cardSize: string;
	    query: AssetQuery;
	
	    static createFrom(source: any = {}) {
	        return new Preferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.theme = source["theme"];
	        this.view = source["view"];
	        this.cardSize = source["cardSize"];
	        this.query = this.convertValues(source["query"], AssetQuery);
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
	export class Category {
	    id: string;
	    name: string;
	    position: number;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.position = source["position"];
	        this.count = source["count"];
	    }
	}
	export class Bootstrap {
	    systemLocale: string;
	    categories: Category[];
	    avatarTags: Tag[];
	    freeTags: Tag[];
	    preferences: Preferences;
	
	    static createFrom(source: any = {}) {
	        return new Bootstrap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.systemLocale = source["systemLocale"];
	        this.categories = this.convertValues(source["categories"], Category);
	        this.avatarTags = this.convertValues(source["avatarTags"], Tag);
	        this.freeTags = this.convertValues(source["freeTags"], Tag);
	        this.preferences = this.convertValues(source["preferences"], Preferences);
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
	
	export class CategoryDraft {
	    id: string;
	    name: string;
	    position: number;
	
	    static createFrom(source: any = {}) {
	        return new CategoryDraft(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.position = source["position"];
	    }
	}
	export class ImportDecision {
	    rowKey: string;
	    action: string;
	    targetAssetId: string;
	    targetRevision: number;
	    resolvedDraft: AssetDraft;
	
	    static createFrom(source: any = {}) {
	        return new ImportDecision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rowKey = source["rowKey"];
	        this.action = source["action"];
	        this.targetAssetId = source["targetAssetId"];
	        this.targetRevision = source["targetRevision"];
	        this.resolvedDraft = this.convertValues(source["resolvedDraft"], AssetDraft);
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
	export class ImportCommitRequest {
	    previewId: string;
	    decisions: ImportDecision[];
	
	    static createFrom(source: any = {}) {
	        return new ImportCommitRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.previewId = source["previewId"];
	        this.decisions = this.convertValues(source["decisions"], ImportDecision);
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
	
	export class SheetMapping {
	    sheet: string;
	    included: boolean;
	    categoryId: string;
	    newCategoryName: string;
	    boothColumn: string;
	    nameColumn: string;
	    imageColumn: string;
	    avatarColumn: string;
	    notesColumn: string;
	    freeTagsColumn: string;
	    folderColumn: string;
	
	    static createFrom(source: any = {}) {
	        return new SheetMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sheet = source["sheet"];
	        this.included = source["included"];
	        this.categoryId = source["categoryId"];
	        this.newCategoryName = source["newCategoryName"];
	        this.boothColumn = source["boothColumn"];
	        this.nameColumn = source["nameColumn"];
	        this.imageColumn = source["imageColumn"];
	        this.avatarColumn = source["avatarColumn"];
	        this.notesColumn = source["notesColumn"];
	        this.freeTagsColumn = source["freeTagsColumn"];
	        this.folderColumn = source["folderColumn"];
	    }
	}
	export class ImportPreviewRequest {
	    path: string;
	    mappings: SheetMapping[];
	
	    static createFrom(source: any = {}) {
	        return new ImportPreviewRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.mappings = this.convertValues(source["mappings"], SheetMapping);
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
	export class JobRef {
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new JobRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	    }
	}
	export class JobSnapshot {
	    id: string;
	    kind: string;
	    status: string;
	    stage: string;
	    completed: number;
	    total: number;
	    cancellable: boolean;
	    resultJson: string;
	    errorJson: string;
	
	    static createFrom(source: any = {}) {
	        return new JobSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.stage = source["stage"];
	        this.completed = source["completed"];
	        this.total = source["total"];
	        this.cancellable = source["cancellable"];
	        this.resultJson = source["resultJson"];
	        this.errorJson = source["errorJson"];
	    }
	}
	export class MediaDraft {
	    token: string;
	    previewUrl: string;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new MediaDraft(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.previewUrl = source["previewUrl"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	
	
	
	
	export class TagDraft {
	    id: string;
	    kind: string;
	    name: string;
	    aliases: string[];
	
	    static createFrom(source: any = {}) {
	        return new TagDraft(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.aliases = source["aliases"];
	    }
	}
	export class WorkspaceInfo {
	    dataPath: string;
	    schemaVersion: number;
	    assetCount: number;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataPath = source["dataPath"];
	        this.schemaVersion = source["schemaVersion"];
	        this.assetCount = source["assetCount"];
	    }
	}

}

