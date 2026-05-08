# Contributing

Thanks for helping improve makc.

## Development

Run the default checks before opening a pull request:

```sh
bash scripts/check.sh
```

The check script runs formatting, tests, cross-platform compile checks, and
`go vet` for the supported targets.

## Pull Requests

- Keep changes focused and describe the user-visible behavior.
- Add or update tests when changing API behavior, backend behavior, parsing, or
  timing helpers.
- Update README or GoDoc when adding exported API.
- Mention any platform that could not be tested locally.

## Platform Notes

Some backends require OS permissions or an interactive desktop session:

- macOS event injection requires Accessibility permission.
- Linux `/dev/uinput` access may require group or udev configuration.
- Linux evdev listening requires readable `/dev/input/event*` devices.
- Windows smoke testing should run in an interactive user session.
