# gemoji-json

A maintained, up-to-date `emoji.json` database extracted from [github/gemoji](https://github.com/github/gemoji) and [Unicode Emoji](https://unicode.org/Public/emoji/latest/emoji-test.txt).

This repo exists because the upstream gemoji repository is [effectively unmaintained](https://github.com/github/gemoji/pull/303#issuecomment-2537285792) and has not been updated past Unicode 15.0. This fork tracks the latest Unicode emoji releases.

## Download

**[emoji.json](https://raw.githubusercontent.com/delthas/gemoji-json/master/emoji.json)** (right-click, Save As)

## Format

Each entry in the JSON array represents an emoji:

```json
[
  {
    "emoji": "😀"
  , "description": "grinning face"
  , "category": "Smileys & Emotion"
  , "aliases": [
      "grinning"
    ]
  , "tags": [
      "smile"
    , "happy"
    ]
  , "unicode_version": "6.1"
  , "ios_version": "6.0"
  }
, {
    "emoji": "🫩"
  , "description": "face with bags under eyes"
  , "category": "Smileys & Emotion"
  , "aliases": [
      "face_with_bags_under_eyes"
    ]
  , "tags": [
    ]
  , "unicode_version": "16.0"
  }
]
```

| Field | Description |
|-------|-------------|
| `emoji` | The emoji character(s) |
| `description` | Human-readable description from the Unicode spec |
| `category` | One of 9 Unicode emoji categories |
| `aliases` | Short names (e.g. `:grinning:` on GitHub) |
| `tags` | Search keywords |
| `unicode_version` | Unicode version that introduced this emoji |
| `ios_version` | iOS version that added support (existing emoji only) |
| `skin_tones` | `true` if the emoji supports skin tone modifiers |

## Development

To regenerate `emoji.json` from the latest Unicode emoji data:

```sh
go run . > emoji.json
```

This fetches the latest `emoji-test.txt` from unicode.org, merges it with the existing `emoji.json` (preserving curated aliases and tags), and outputs the updated file.

## License

[MIT](LICENSE)
