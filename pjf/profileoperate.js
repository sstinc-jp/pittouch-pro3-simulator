function ProFileOperate() {
    return ProFileOperateImpl.getInstance();
}

class ProFileOperateImpl {
    constructor() {
        this.write = this.write.bind(this);
        this.read = this.read.bind(this);
    }

    static getInstance() {
        if (!ProFileOperateImpl.instance) {
            ProFileOperateImpl.instance = new ProFileOperateImpl();
        }
        return ProFileOperateImpl.instance;
    }

    write(param) {
        let req = {
            fileName: param.fileName,
            data: param.data,
            isAppend: param.isAppend ?? false,
        }
        const xhr = new XMLHttpRequest();
        xhr.open("POST", "/pjf/api/writeFile", false);
        xhr.setRequestHeader("Content-Type", "application/json");
        xhr.send(JSON.stringify(req));

        return 0;
    }

    read(param) {
        let req = {
            fileName: param.fileName,
        }
        const xhr = new XMLHttpRequest();
        xhr.open("POST", "/pjf/api/readFile", false);
        xhr.setRequestHeader("Content-Type", "application/json");
        xhr.send(JSON.stringify(req));

        if (xhr.status === 200) {
            return xhr.responseText
        } else {
            return "";
        }
    }
}