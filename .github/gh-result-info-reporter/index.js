const core = require("@actions/core")
const { Octokit } = require("@octokit/rest")

try {
    // Add `|| VALUE` to the ends of these when debugging while not running the action
    const id = core.getInput("run_id");
    const repo = core.getInput("repo");
    const owner = core.getInput("owner");
    const token = core.getInput("token");
    const octokit = new Octokit({
        auth: token
    });

    (async () => {
        let result = await octokit.rest.actions.listJobsForWorkflowRun({ 
            owner, 
            repo, 
            run_id: id 
        });
        let data = result["data"];
        let job_id = data["jobs"][0]["id"]
        const url = data["jobs"][0]["html_url"]

        console.log(result);
        console.log("--------");
        console.log(data);
        console.log("--------");

        result = await octokit.rest.checks.listAnnotations({ 
            owner, 
            repo, 
            check_run_id: job_id 
        });

        const errorJson = result["data"];
        let error_out = "-----------\n";
        for (const item of errorJson) {
            error_out += item["message"] + "\n";
        }
        error_out += "-----------";

        console.log(url);
        console.log(error_out);

        core.setOutput("url", url);
        core.setOutput("error_info", error_out);
    })();
} catch (e) {
    console.error(e)
}