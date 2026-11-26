# Tasks

## 1. Flags anywhere for epic

- [x] 1.1 `parseInterspersed` parses value and boolean flags anywhere and honours `--`
- [x] 1.2 `epic` uses it; the title is every positional word joined
- [x] 1.3 Tests cover title-first, flags-first, flags between title words, and `--`
- [x] 1.4 A title word that is a registered flag (an unsplit shell variable such as `" -child a -child b"`) fails instead of becoming part of the title; text after `--` is exempt
- [x] 1.5 The arg-check guard accepts `parseInterspersed`, so epic needs no exemption

## 2. Slug resolves to the declared target

- [x] 2.1 `changeRef` prefers a ref whose key is a declared `specsync.yml` target
- [x] 2.2 `link`/`epic` slug resolution uses it
- [x] 2.3 Test with a stray ref that sorts before the declared target
- [x] 2.4 A cache holding only a stray ref counts as no ref: `link` says so, `epic` syncs the change first
