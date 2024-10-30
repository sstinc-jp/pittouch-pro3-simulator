function ProOperate() {
    return ProOperateImpl.getInstance();
}

class ProOperateImpl {
    constructor() {
        this.productType = "pro3";
        this.startKeypadListen = this.startKeypadListen.bind(this);
        this.stopKeypadListen = this.stopKeypadListen.bind(this);
        this.setKeypadDisplay = this.setKeypadDisplay.bind(this);
        this.getKeypadDisplay = this.getKeypadDisplay.bind(this);
        this.setKeypadLed = this.setKeypadLed.bind(this);
        this.getKeypadLed = this.getKeypadLed.bind(this);
        this.getKeypadConnected = this.getKeypadConnected.bind(this);
        this.playSound = this.playSound.bind(this);
        this.stopSound = this.stopSound.bind(this);
        this.startCommunication = this.startCommunication.bind(this);
        this.stopCommunication = this.stopCommunication.bind(this);
        this.startEventListen = this.startEventListen.bind(this);
        this.stopEventListen = this.stopEventListen.bind(this);
        this.getNetworkStat = this.getNetworkStat.bind(this);
        this.startHttpRequestListen = this.startHttpRequestListen.bind(this);
        this.stopHttpRequestListen = this.stopHttpRequestListen.bind(this);
        this.sendHttpResponse = this.sendHttpResponse.bind(this);
        this.getTerminalID = this.getTerminalID.bind(this);
        this.getFirmwareVersion = this.getFirmwareVersion.bind(this);
        this.getContentsSetVersion = this.getContentsSetVersion.bind(this);
        this.removeAllWebSQLDB = this.removeAllWebSQLDB.bind(this);
        this.setDate = this.setDate.bind(this);
        this.reboot = this.reboot.bind(this);
        this.shutdown = this.shutdown.bind(this);
        this.setDisplayBrightness = this.setDisplayBrightness.bind(this);
        this.getDisplayBrightness = this.getDisplayBrightness.bind(this);
        this.clearSettingPassword = this.clearSettingPassword.bind(this);

        this.networkStat = 2; // LAN
        this.startCommunicationOnEvent = undefined;
        this.startEventListenOnEvent = undefined;
        this.startKeypadListenOnEvent = undefined;

        this.connectWebSocket();
    }


    static getInstance() {
        if (!ProOperateImpl.instance) {
            ProOperateImpl.instance = new ProOperateImpl();
        }
        return ProOperateImpl.instance;
    }

    connectWebSocket() {
        const webSocket = new WebSocket(`ws://${location.host}/pjf/api/eventNotification`);
        webSocket.onopen = (event) => {
            window.addEventListener("beforeunload", (event) => {
                webSocket.close();
            });
        };
        webSocket.onmessage = (event) => {
            this.handleEvent(event.data);
        };
        webSocket.onerror = (event) => {
        };
        webSocket.onclose = (event) => {
            // openできなかった場合、また接続が切れた場合は自動で再接続を試みる。
            setTimeout(() => {
                this.connectWebSocket();
            }, 500);
        };
    }

    handleEvent(data) {
        let json;
        try {
            json = JSON.parse(data);
        } catch (e) {
            return;
        }
        switch (json["api"]) {
            case "startCommunication":
                if (this.startCommunicationOnEvent) {
                    this.startCommunicationOnEvent(json["eventCode"], json["responseObject"]);
                }
                break;
            case "startKeypadListen":
                if (this.startCommunicationOnEvent) {
                    this.startKeypadListenOnEvent(json["eventCode"]);
                }
                break;
            case "startEventListen":
                if (this.startEventListenOnEvent) {
                    let eventCode = json["eventCode"];
                    switch (eventCode) {
                        case 0: // disconnected
                            this.networkStat = 0;
                            break;
                        case 1: // mobile
                            this.networkStat = 1;
                            break;
                        case 2: // LAN
                            this.networkStat = 2;
                            break;
                        case 6: // WLAN
                            this.networkStat = 3;
                            break;
                    }
                    this.startEventListenOnEvent(eventCode);
                }
                break;
        }
    }


    startKeypadListen(param) {
        if (param instanceof Object && param["onEvent"] instanceof Function) {
            this.startKeypadListenOnEvent = param["onEvent"];
        }
        return 0;
    }

    stopKeypadListen() {
        this.startKeypadListenOnEvent = undefined;
        return 0;
    }

    setKeypadDisplay(param) {
        return 0;
    }

    getKeypadDisplay() {
        return {
            firstList: "",
            secondLine: "",
        };
    }

    setKeypadLed(param) {
        return 0;
    }

    getKeypadLed() {
        return "000000";
    }

    getKeypadConnected() {
        return 1; // 接続中
    }

    playSound(param) {
        return 9999; // 音声のID
    }

    stopSound(param) {
        return 0;
    }

    startCommunication(param) {
        if (param instanceof Object && param["onEvent"] instanceof Function) {
            this.startCommunicationOnEvent = param["onEvent"];
        }
        return 0;
    }

    stopCommunication() {
        this.startCommunicationOnEvent = undefined;
        return 0;
    }

    startEventListen(param) {
        if (param instanceof Object && param["onEvent"] instanceof Function) {
            this.startEventListenOnEvent = param["onEvent"];
        }
        return 0;
    }

    stopEventListen() {
        this.startEventListenOnEvent = undefined;
        return 0;
    }

    getNetworkStat() {
        return this.networkStat;
    }

    startHttpRequestListen(param) {
        return 0;
    }

    stopHttpRequestListen() {
        return 0;
    }

    sendHttpResponse(param) {
        return 0;
    }

    getTerminalID() {
        return "00000000"
    }

    getFirmwareVersion() {
        return "5.00r000000"
    }

    getContentsSetVersion() {
        return "000"
    }

    removeAllWebSQLDB() {
        const xhr = new XMLHttpRequest();
        xhr.open("GET", "/pjf/api/removeAllWebSQLDB", false);
        xhr.send(null);
    }

    setDate(year, month, day, hour, minute, second) {
        return 0;
    }

    reboot() {
        return 0;
    }

    shutdown() {
        return 0;
    }

    setDisplayBrightness(param) {
        return 0;
    }

    getDisplayBrightness() {
        return 0;
    }

    clearSettingPassword() {
        return 0;
    }
}
