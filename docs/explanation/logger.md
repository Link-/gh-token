# Logger

We're using the Logrus package for logging within the CLI. The logger is built in the `utility.go` file and is used by all of the CLI commands.

Example:
```golang
// Build the logger and use it for any output
logger := NewLogger(cli.Logging.Level, cli.Logging.Type)
logger.Debugf("InstallationsCmd called with AppID %v, KeyFile %v", cmd.AppID, cmd.KeyFile)
```

## Logging Levels

The [logging levels supported by Logrus](https://github.com/sirupsen/logrus#level-logging) are: Trace, Debug, Info, Warn, Error, Fatal, Panic.

After building the logger in the above example you could use any of the logging levels to output to the console.

Example:
```golang
logger.Trace("Something very low level.")
logger.Debug("Useful debugging information.")
logger.Info("Something noteworthy happened!")
logger.Warn("You should probably take a look at this.")
logger.Error("Something failed but I'm not quitting.")
// Calls os.Exit(1) after logging
logger.Fatal("Bye.")
// Calls panic() after logging
logger.Panic("I'm bailing.")
```

**Note:** Whilst the Trace logging level is available with Logrus, I've not included the trace level in the CLI as a flag as I believe debug should cover most of what a user should see.

## Logging Types

The support logging types are either console or JSON. The default is console.

See [Debug Logging](../how-to/debug-logging.md) for more information on how to use the logging types.
