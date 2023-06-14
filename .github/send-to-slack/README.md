# send to slack

A action to send data about VisibleV8 workflow runs to Slack.

## How to edit the `dist/index.js` file

> **Warning** Don't edit the `dist/index.js` file

The source of the dist/index.js file is the index.js file here.
According to GitHubs guidelines, users are not supposed to upload the npm dependencies when creating actions like this
So, they wanted us to use `@vercel/ncc` to compile the dependencies into one large file, hence the dist/index.js

## Build

After making changes to `index.js` build with the following commands before commiting:

```sh
npm i -g @vercel/ncc
ncc build index.js
```
