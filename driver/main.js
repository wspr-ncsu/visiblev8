#!/usr/bin/env node
"use strict";
// Bare-bones example of a Puppeteer-based driver script for Chromium/VisibleV8
//-----------------------------------------------------------------------------
const puppeteer = require('puppeteer');

// Tuning knob: which Chromium executable to use?
const CHROME_EXE = process.env.CHROME_EXE || puppeteer.executablePath();

// Entrypoint logic: fire up Chromium, visit <url>, capture a screenshot and VV8 logs, and shut down
async function main(url) {
    try {
        const launchConfig = {
            executablePath: CHROME_EXE,
            args: ["--no-sandbox"], // currently required for VisibleV8 (so it can create log files)
        };
        console.log("launching browser...");
        const browser = await puppeteer.launch(launchConfig);

        console.log("opening new tab...");
        const page = await browser.newPage();

        console.log(`navigating to "${url}"...`)
        await page.goto(url);

        const screenshotPath = "screenshot.png";
        console.log(`saving screenshot to "${screenshotPath}"...`);
        await page.screenshot({
            path: "screenshot.png"
        });

        console.log("shutting down...");
        await browser.close();

        process.exit(0);
    } catch (err) {
        console.error(err);
        process.exit(1);        
    }
}

// Parse URL argument and invoke main function
const program = require('commander');
program
    .arguments("<url>")
    .action(main)
    .parse(process.argv);
