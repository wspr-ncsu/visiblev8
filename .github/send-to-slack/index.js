const core = require("@actions/core");
const fetch = require("node-fetch");
(async () => {
    const url = core.getInput("url");
    const error_out = core.getInput("error_info");
    const slack_url = core.getInput("webhook");
    const chrome_version = core.getInput("chromeVersion");
    const commit = core.getInput("currentGitCommit");
    const message = 'VisibleV8 build ' + commit + ' for Chromium version '+ chrome_version +' failed. Check the <' + url + '|latest logs> for the full details.\n\n*Errors*```' + error_out + '```'
    const resp = await fetch(slack_url, {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({
            text: message
        })
    });
    console.log(await resp.text());

})();