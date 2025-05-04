# packwiz-nxt

This is a rewrite of [packwiz](https://github.com/packwiz/packwiz). 
This repo focuses on changing the codebase so that packwiz can be used as a 
library without needing to write to the file system.
The original CLI functionality is maintained.

Note:
This fork does not include a Curseforge API key in its source code. 
You can apply for one [here](https://forms.monday.com/forms/dce5ccb7afda9a1c21dab1a1aa1d84eb?r=use1).
You must supply it with one of these methods:

- if building the project locally, include the ldflag `-X main.CfApiKey=<base64-encoded-key>`
  - using the `make` file: `CF_API_KEY=<base-64-encoded-key> make`
- if using as a library, call `config.SetCurseforgeApiKey(<base-64-encoded-key>)` at some point in your code

---

**From the original repo:**

> packwiz is a command line tool for creating Minecraft modpacks. 
Instead of managing JAR files directly, packwiz creates TOML metadata files 
which can be easily version-controlled and shared with git (see an example 
pack [here](https://github.com/packwiz/packwiz-example-pack)). You can then [export it to a CurseForge or Modrinth modpack](https://packwiz.infra.link/tutorials/hosting/curseforge/), 
or [use packwiz-installer](https://packwiz.infra.link/tutorials/installing/packwiz-installer/) for an auto-updating MultiMC instance.

---

## Development

Local development is facilitated through the make file and its commands.

```shell
# development build
$ make

# tests
$ make test

# lint and autoformat
$ make lint
$ make fmt
```