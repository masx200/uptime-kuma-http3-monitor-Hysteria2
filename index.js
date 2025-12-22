#!/usr/bin/env node
require("child_process").exec(
  "bash warp.sh",
  {
    stdio: "inherit",
    env: {},
  },
  (error, stdout, stderr) => {
    console.error(error);
    console.error(stderr);
    console.log(stdout);
  }
);

require("child_process").execSync("bash start.sh", {
  stdio: "inherit",
  env: {
    REALITY_PORT: 20143,
    HY2_PORT: 20143,
  },
});
