```sh
* _____ _   *_   _______ *  _      *  *    **   *
 / ____| |* | | |__   __|  | |  *       *            *
| | *__| |_*| | *  | | ___ | | _____*_ __  *     *
| | |_ |* __ *|    |*|/ _ \| |/ / _ \ '_ \     *   *
| |__| | |  | | *  | | (_)*|   <  __/ | | |  *
 \_____|_|  |_|    |_|\___/|_|\_\___|_| |_|   *
```

> Create an installation access token for a GitHub app from your terminal

[Creates an installation access token](https://docs.github.com/en/rest/reference/apps#create-an-installation-access-token-for-an-app) that enables a GitHub App to make authenticated API requests for the app's installation on an organization or individual account. Installation tokens expire 10 minutes from the time you create them. Using an expired token produces a status code of `401 - Unauthorized`, and requires creating a new installation token.

![ghtoken demo](./images/ghtoken.png)

## Installation

Download `ghtoken` [from the main branch](https://github.com/Link-/github-app-bash/blob/main/ghtoken)

### wget

```sh
# Download a file, name it ghtoken then do a checksum
wget -O ghtoken \
    https://raw.githubusercontent.com/Link-/github-app-bash/main/ghtoken && \
    echo "b1bd0469d77666d9dec92cc8cd8c67c81794832a5b39dafb5f6c809f4e1ab12d  ghtoken" | \
    shasum -c -
```

### curl

```sh
# Download a file, name it ghtoken following [L]ocation redirects, and 
# automatically [C]ontinuing (resuming) a previous file transfer then 
# do a checksum
curl -o ghtoken \
     -O -L -C  - \
     https://raw.githubusercontent.com/Link-/github-app-bash/main/ghtoken && \
     echo "b1bd0469d77666d9dec92cc8cd8c67c81794832a5b39dafb5f6c809f4e1ab12d  ghtoken" | \
     shasum -c -
```

## Usage

Compatible with [GitHub Enterprise Server](https://github.com/enterprise).

```text

Usage:
  ghtoken generate --key /tmp/crt.key --duration 10

Options:
  -k | --key <key>  Path to a PEM-encoded certificate and key. (Required)
  -i | --app_id <id>  GitHub App Id
  -d | --duration <duration>  The duration of the token in minutes. (Default = 10)
  -h | --hostname <hostname>  The API URL of GitHub. (Default = api.github.com)

Description:
  Generates a JWT signed with the supplied key and fetches an
  installation token

```
