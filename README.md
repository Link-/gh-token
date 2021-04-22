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

### Parameters
