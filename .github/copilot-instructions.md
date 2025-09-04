## Project structure

[Creates an installation access token](https://docs.github.com/en/rest/reference/apps#create-an-installation-access-token-for-an-app) to make authenticated API requests to github.com.

```
.
└── internal: the core package where this utility is implemented
```

## Coding instructions

- Read the `Makefile` in the root directory for a list of targets and commands you can run
- Add the necessary package dependencies before running unit tests, especially new mocks
- Attempt to edit the files directly in vscode instead of relying on CLI commands like `sed` to find and replace. Use `sed` as a last restort or when it is more efficient
- When creating new unit tests, append `_test.go` to the basename of the file that the unit tests should be covering.
- When implementing unit tests, adopt the same style of other tests in the same test suite and file. If tabular tests are used write the new tests in that same style. If there are no tests in the same suite, look at the other tests in the same package.
- Create all unit testing fixtures in the folder `fixtures` which must be a subdirectory of where the test files are located.
- When implementing unit tests make sure to read the function you're implementing the test for first.
- When updating unit tests make sure to read the function you're updating the tests for first. Fixing when and how often certain mocks are called might be sufficient to fix the tests.
- In tabular unit tests, the `description` or `name` of the test case is a string that might include white spaces. When searching or running a specific test, white spaces need to be substituted with `_`.

## git operations

- Never stage or commit changes without prompting the user for approval
- Start commit messages with a verb (`Add`, `Update`, `Fix` etc.)
- Do not use `feat:`, `chore:` or anything in that style for commit messages
- Add details of what was changed to the body of the commit message. Be concise.
- Never use: `git pull` `git push` `git merge` `git rebase` `git rm`