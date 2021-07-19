'use strict';

const fs = require('fs');

const pptr = require('puppeteer-core')
const rimraf = require('rimraf');
const Xvfb = require('xvfb');



class Session {
    constructor() {
        this._launch = true;
        this._options = {
            defaultViewport: null,
            args: [],
            headless: false,
        };
        this._cleanupStack = [];
    }

    useBinary(chromeExe) {
        this._launch = true;
        this._options.executablePath = chromeExe;
        return this;
    }

    useXvfb() {
        const xServer = new Xvfb();
        xServer.startSync();
        this._cleanupStack.push(() => {
            try {
                xServer.stopSync();
            } catch (err) {
                console.error("error shutting down Xvfb:", err);
            }
        });
        return this;
    }

    useProxyServer(url) {
        url = new URL(url);
        this._options.args.push(`--proxy-server=${url.toString()}`);
        if (url.protocol === "socks5") {
            this._options.args.push(`--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE ${url.hostname}`);
        }
    }

    useTempProfile(optPrefix) {
        const tempDir = fs.mkdtempSync(optPrefix || "pptr_");
        this._cleanupStack.push(() => {
            try {
                rimraf.sync(tempDir);
            } catch (err) {
                console.error("error deleting temp profile:", err);
            }
        });
        return this;
        /*process.on('exit', () => {
            console.error(`wiping out temp dir: ${tempDir}`);
            rimraf.sync(tempDir);
        });*/
    }

    async run(asyncHandler) {
        const browser = await pptr.launch(this._options);
        try {
            return await asyncHandler(browser);
        } finally {
            await browser.close().catch(err => console.error("error closing browser:", err));
            this._cleanupStack.reverse().forEach(cb => cb());
        }
    }
}

module.exports = { Session }