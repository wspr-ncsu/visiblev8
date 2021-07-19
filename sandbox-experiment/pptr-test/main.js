#!/usr/bin/env node
const am = require('am');

const { Session } = require('./lib/launch');
const { timeoutIn } = require('./lib/utils');

const CHROME_EXE = process.env.CHROME_EXE || '/usr/bin/google-chrome'
const USE_XVFB = !!process.env.USE_XVFB
const NAV_TIMEOUT = 30.0 * 1000
const NAV_COMPLETE_EVENT = 'load'
const MAX_VISIT_TIME = 60.0 * 1000


const doExeConTrackerVisit = async (browser, targetUrl) => {
    const sessionSet = new WeakSet();
    const instrumentSession = async (cdp) => {
        const sessionId = cdp._sessionId;
        const targetType = cdp._targetType;
        if (sessionSet.has(cdp)) {
            console.log("old session", sessionId, targetType);
            return;
        }
        console.log("new session", sessionId, targetType);

        if (["page", "iframe"].includes(targetType)) {
            await Promise.all([
                cdp.send('Page.enable'),
                cdp.send('Runtime.enable'),
            ]);
        }
        await cdp.send('Target.setAutoAttach', {
            autoAttach: true,
            waitForDebuggerOnStart: true,
            flatten: true,
        });
        cdp.on('Target.attachedToTarget', async (params) => {
            const { sessionId, targetInfo } = params;
            //console.log("COLLECTOR-DEBUG: Target.attachedToTarget:", sessionId, targetInfo.type, targetInfo.targetId);
            const cdp = browser._connection._sessions.get(sessionId);
            console.log('ATTACH:', targetInfo);
            await instrumentSession(cdp);
        });
        cdp.on('Runtime.executionContextCreated', async (params) => {
            const { context } = params;
            console.log('EXECON:', context);
        });
        cdp.on('Page.frameNavigated', async (params) => {
            const { frame, type } = params;
            console.log('FRAME:', type, frame);
        });
        try {
            await cdp.send('Runtime.runIfWaitingForDebugger');
        } catch { }
        //console.log(`DONE INSTRUMENTING SESSION ${sessionId}`);
    };

    const rootSession = await browser.target().createCDPSession();
    await instrumentSession(rootSession);
    
    const page = await browser.newPage();
    await page.goto(targetUrl, {
        timeout: NAV_TIMEOUT,
        waitUntil: NAV_COMPLETE_EVENT,
    });
    console.log("LOADED!");
};

am(async function main(targetUrl) {
    const session = new Session();

    session.useBinary(CHROME_EXE).useTempProfile();
    if (USE_XVFB) {
        session.useXvfb();
    }

    let exitStatus = 0;
    await session.run(async (browser) => {
        return Promise.race([
            doExeConTrackerVisit(browser, targetUrl),
            timeoutIn(MAX_VISIT_TIME),
        ]);
    }).catch(err => {
        console.error("error while browsing:", err);
        exitStatus = 1;
    });
    console.log("all done");
    process.exit(exitStatus);
});