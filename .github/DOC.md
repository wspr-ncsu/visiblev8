# Runner documentation

VisibleV8 uses a array of three self-hosted runners on a single computer on the ncsu network. These workers are labelled as follows:
- `vv8-builder`
- `postprocessor-builder`
- `workflow-finish-runner`
corresponding to each of the workflows. This is done to make sure that even if one workflow fails, the other workflows are able to continue building without any difficulty.

## Running the runners

The runner are using Github's default runner configuration. The `./run.sh` script is being run with the following command:

```sh
nohup ./run.sh &> ./output.log &
```
