# packwiz-nxt

This is a rewrite of [packwiz](https://github.com/packwiz/packwiz). 
This repo focuses on changing the codebase so that packwiz can be used as a 
library without needing to write to the file system.
The original CLI functionality is maintained.

Note:
This fork does not include a Curseforge API key in its source code. 
You must supply it with one of these methods:

- if building the project, include the ldflag `-X main.CfApiKey=<key>`
- if using as a library, set `config.CfApiKey` variable at some point

Prebuilt binary releases for this repo will include a Curseforge API key.

---

packwiz is a command line tool for creating Minecraft modpacks. 
Instead of managing JAR files directly, packwiz creates TOML metadata files 
which can be easily version-controlled and shared with git (see an example 
pack [here](https://github.com/packwiz/packwiz-example-pack)). You can then [export it to a CurseForge or Modrinth modpack](https://packwiz.infra.link/tutorials/hosting/curseforge/), 
or [use packwiz-installer](https://packwiz.infra.link/tutorials/installing/packwiz-installer/) for an auto-updating MultiMC instance.
