```sh
* _____ _   *_   _______ *  _      *  *    **   *
 / ____| |* | | |__   __|  | |  *       *            *
| | *__| |_*| | *  | | ___ | | _____*_ __  *     *
| | |_ |* __ *|    |*|/ _ \| |/ / _ \ '_ \     *   *
| |__| | |  | | *  | | (_)*|   <  __/ | | |  *
 \_____|_|  |_|    |_|\___/|_|\_\___|_| |_|   *
```

> Create an installation access token for a GitHub app from your terminal

[Creates an installation access token](https://docs.github.com/en/rest/reference/apps#create-an-installation-access-token-for-an-app) that enables a GitHub App to make authenticated API requests for the app's installation on an organization or individual account. Installation tokens expire 1 hour from the time you create them. Using an expired token produces a status code of `401 - Unauthorized`, and requires creating a new installation token.

![ghtoken demo](./images/ghtoken.png)

## Installation

Download `ghtoken` [from the main branch](https://github.com/Link-/github-app-bash/blob/main/ghtoken)

### wget

```sh
# Download a file, name it ghtoken then do a checksum
wget -O ghtoken \
    https://raw.githubusercontent.com/Link-/github-app-bash/main/ghtoken && \
    echo "48ea32970b0ac57b2f1a3b1dbbef2c99b19cb88d31e9e65108bef1ec4eafe086  ghtoken" | \
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
     echo "48ea32970b0ac57b2f1a3b1dbbef2c99b19cb88d31e9e65108bef1ec4eafe086  ghtoken" | \
     shasum -c -
```

## Usage

Compatible with [GitHub Enterprise Server](https://github.com/enterprise).

```text

Usage:
  ghtoken generate --key /tmp/private-key.pem --app_id 112233

Options:
  -k | --key <key>  Path to a PEM-encoded certificate and key. (Required)
  -b | --base64_key <key> Base64 encoded PEM certificate and key. (Optional)
  -i | --app_id <id>  GitHub App Id
  -d | --duration <duration>  The duration of the token in minutes. (Default = 10)
  -h | --hostname <hostname>  The API URL of GitHub. (Default = api.github.com)
  -j | --install_jwt_cli  Install jwt-cli (dependency) on the current system. (Optional)

Description:
  Generates a JWT signed with the supplied key and fetches an
  installation token
```

### Examples in the Terminal

#### Run `ghtoken` assuming `jwt-cli` is already installed

```sh
# Assumed starting point
.
├── .keys
│   └── private-key.pem
├── README.md
└── ghtoken

1 directory, 3 files

# Run ghtoken
$ ghtoken generate \
    --key ./.keys/private-key.pem \
    --app_id 1122334 \
    | jq

{
  "token": "ghs_g7___MlQiHCYI__________7j1IY2thKXF",
  "expires_at": "2021-04-28T15:53:44Z"
}
```

#### Run `ghtoken` and install `jwt-cli`

```sh
# Assumed starting point
.
├── .keys
│   └── private-key.pem
├── README.md
└── ghtoken

1 directory, 3 files

# Run ghtoken and add --install_jwt_cli
$ ghtoken generate \
    --key ./.keys/private-key.pem \
    --app_id 1122334 \
    --install_jwt_cli \
    | jq

{
  "token": "ghs_8Joht_______________bLCMS___M0EPOhJ",
  "expires_at": "2021-04-28T15:55:32Z"
}

# jwt-cli will be downloaded in the same directory
.
├── .keys
│   └── private-repo-checkout.2021-04-22.private-key.pem
├── README.md
├── ghtoken
└── jwt
```

#### Run `ghtoken` and pass the key as a base64 encoded variable

```sh
# Assumed starting point
.
├── README.md
└── ghtoken

1 directory, 2 files

# Run ghtoken and add --install_jwt_cli
$ ghtoken generate \
    --base64_key $(printf "%s" $APP_KEY | base64) \
    --app_id 1122334 \
    --install_jwt_cli \
    | jq

{
  "token": "ghs_GxVel5cp__________DOaCv8eDs___2l94Ta",
  "expires_at": "2021-04-28T16:30:59Z"
}
```

#### Run `ghtoken` with GitHub Enterprise Server

```sh
# Assumed starting point
.
├── .keys
│   └── private-key.pem
├── README.md
└── ghtoken

1 directory, 3 files

# Run ghtoken and specify the --hostname
$ ghtoken generate \
    --key ./.keys/private-key.pem \
    --app_id 2233445 \
    --install_jwt_cli \
    --hostname "github.example.com" \
    | jq

{
  "token": "v1.bb1___168d_____________1202bb8753b133919",
  "expires_at": "2021-04-28T16:01:05Z"
}
```

### Example in a workflow

1. You need to create a secret to store the applications private key securely (this can be an organization or a repository secret):
    ![Create private key secret](images/create_secret.png)

1. The secret needs to be provided as an environment variable then encoded into base64 as show in the workflow example:

```yaml
name: Create access token via GitHub Apps Workflow

on:
  workflow_dispatch:

jobs:
  Test:
    # The type of runner that the job will run on
    runs-on: [ self-hosted ]

    # Steps represent a sequence of tasks that will be executed as part of the job
    steps:
    - name: "Download ghtoken"
      run: |
        curl -o ghtoken \
             -O -L -C  - \
             https://raw.githubusercontent.com/Link-/github-app-bash/main/ghtoken && \
             echo "48ea32970b0ac57b2f1a3b1dbbef2c99b19cb88d31e9e65108bef1ec4eafe086  ghtoken" | \
             shasum -c - && \
             chmod a+x ./ghtoken
    - name: "Create access token"
      run: |
        ./ghtoken generate \
          --base64_key $(printf "%s" "$APP_PRIVATE_KEY" | base64 -w 0) \
          --app_id 3 \
          --install_jwt_cli \
          --hostname "github.example.com" \
          | jq
      env:
        APP_PRIVATE_KEY: ${{ secrets.APP_KEY }}
```
