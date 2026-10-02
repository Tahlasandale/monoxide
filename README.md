# monoxide

A third-party, open-source ProtonMail bridge. For power users only, designed to
run on a server.

monoxide supports CardDAV, CalDAV, IMAP and SMTP.

> **Heads up:** this repository is *not* the original project. See
> [Provenance](#provenance) below.

Rationale:

* No GUI, only a CLI (so it runs in headless environments)
* Standard-compliant (we don't care about Microsoft Outlook)
* Fully open-source

## How does it work?

monoxide is a server that translates standard protocols (SMTP, IMAP, CardDAV,
CalDAV) into ProtonMail API requests. It allows you to use your preferred
e-mail clients and `git-send-email` with ProtonMail.

    +-----------------+             +-------------+  ProtonMail  +--------------+
    |                 | IMAP, SMTP  |             |     API      |              |
    |  E-mail client  <------------->  monoxide   <--------------> | ProtonMail  |
    |                 |             |             |              |              |
    +-----------------+             +-------------+              +--------------+

## Provenance

monoxide is a fork of **[acheong08/ferroxide](https://github.com/acheong08/ferroxide)**,
itself a fork of **[emersion/hydroxide](https://codeberg.org/emersion/hydroxide)**.

Changes relative to `ferroxide`:

* **Human-verification (CAPTCHA) support.** Proton's API answers unknown
  clients with `Code: 9001` and requires a human-verification challenge to be
  solved before authentication is allowed. This fork parses the challenge
  details from the error response, prints the `verify.proton.me` URL, accepts
  the resulting token, and replays `/auth` carrying the
  `x-pm-human-verification-token` and `x-pm-human-verification-token-type`
  headers. See [Human verification](#human-verification).
* `mailread` helper to list and read messages from a terminal.
* Module path renamed to `github.com/Tahlasandale/monoxide`; the configuration
  directory is now `~/.config/monoxide`.

Changes inherited from `ferroxide`:

* CalDAV support
* Tor and SOCKS proxy support
* Custom configuration directory

### Legal note

This project is distributed under the MIT license, unchanged from upstream —
see [LICENSE](LICENSE). The MIT license of the original work requires that the
copyright notice and permission notice be retained in all copies or substantial
portions of the Software.

ProtonMail's API is not a public API. Proton does not officially support
third-party clients, and the human-verification flow implemented here exists
specifically to work around a control Proton put in place deliberately. Using
it may conflict with Proton's Terms of Service and can result in your account
being restricted or disabled. **Consider
[Proton Mail Bridge](https://proton.me/mail/download), the officially
supported alternative.**

## Setup

### Before you clone

* **Go 1.23 minimum, 1.24 recommended.** `go.mod` pins `go 1.23.0` with
  `toolchain go1.24.2`.
* **GitHub Actions is not enabled on this repository**, so there are no CI
  artifacts. Build from source.
* The binary is named **`monoxide`**, matching the module and the
  `~/.config/monoxide` configuration directory.

### Installing

```shell
git clone https://github.com/Tahlasandale/monoxide.git
cd monoxide
go build ./cmd/monoxide
```

Optionally put it on your `PATH`:

```shell
go build -o ~/.local/bin/monoxide ./cmd/monoxide
```

Then login to ProtonMail so that monoxide can retrieve e-mails:

```shell
./monoxide auth <username>
```

Once logged in, a "bridge password" will be printed. **Save it somewhere safe** —
it is not stored anywhere and is required to decrypt your stored credentials.
You can pass it non-interactively with the `HYDROXIDE_BRIDGE_PASS` environment
variable.

Your ProtonMail credentials are stored on disk encrypted with this bridge
password (a 32-byte random password generated when logging in).

### Human verification

Proton may answer `auth` with:

```
[9001] For security reasons, please complete CAPTCHA.
```

When this happens, `auth` prints a verification URL:

```
Proton requires human verification for this login.
Open this URL in your browser and solve the captcha:

    https://verify.proton.me?methods=captcha&token=...

Then copy the token it gives you and paste it below.
```

Open the URL, solve the captcha, paste the token back. The login is then
retried with the verification headers attached. Up to 5 attempts are allowed.

If the response offers no captcha method, `auth` points you at
`https://mail.proton.me` instead.

## Usage

> Don't start monoxide multiple times, instead you can use
> `monoxide serve`. This requires ports 1025 (smtp), 1143 (imap), 8080
> (carddav) and 8081 (caldav).

### SMTP

```shell
monoxide smtp
```

Configure your e-mail client with:

* Hostname: `localhost`
* Port: 1025
* Security: none
* Username: your ProtonMail username
* Password: the bridge password (not your ProtonMail password)

### IMAP

⚠️  **Warning**: IMAP support is work-in-progress. Here be dragons.

Only unencrypted local connections are supported.

```shell
monoxide imap
```

### CardDAV / CalDAV

You must set up an HTTPS reverse proxy to forward requests to `monoxide`.

```shell
monoxide carddav
monoxide caldav
```

### Reading mail from a terminal

`mailread` is a separate binary, build it first:

```shell
go build ./cmd/mailread
```

```shell
# list unread messages
HYDROXIDE_BRIDGE_PASS=... ./mailread -user <username>

# list everything
HYDROXIDE_BRIDGE_PASS=... ./mailread -user <username> -all

# read bodies and mark as read
HYDROXIDE_BRIDGE_PASS=... ./mailread -user <username> -read -mark-read
```

## License

MIT — see [LICENSE](LICENSE). Original work © 2017 emersion.