# Using monoxide with Himalaya

[Himalaya](https://github.com/pimalaya/himalaya) is a terminal mail client. This
guide wires it to a local `monoxide` bridge over IMAP (reading) and SMTP
(sending).

## Install Himalaya

If Himalaya is not already installed:

```shell
curl -sSL https://raw.githubusercontent.com/pimalaya/himalaya/master/install.sh | PREFIX=~/.local sh
```

This lands the binary in `~/.local/bin/himalaya`. Make sure `~/.local/bin` is on
your `PATH`.

## The two traps, read this first

The IMAP and SMTP servers in `monoxide` advertise different SASL mechanisms and
each one fails on the mechanism it advertises. Use the opposite mechanism on each
protocol. Getting this wrong is the single most common reason the setup appears
broken.

| Protocol | Use | Fails with |
| --- | --- | --- |
| IMAP | `imap.sasl.login.*` | `imap.sasl.plain.*` → `AUTHENTICATE PLAIN failed: Invalid access token` |
| SMTP | `smtp.sasl.plain.*` | `smtp.sasl.login.*` → `SMTP AUTH LOGIN failed: 454 4.7.0 sasl: invalid response` |

Both servers speak cleartext and only listen on `127.0.0.1`. That is fine for a
local bridge, but see [Security](#security) before exposing anything.

## 1. Start the bridge

```shell
monoxide serve
```

It listens on four ports. Only the first two are needed for Himalaya:

| Port | Service |
| --- | --- |
| 1025 | SMTP (sending) |
| 1143 | IMAP (reading) |
| 8080 | CardDAV |
| 8081 | CalDAV |

Confirm it is up before going further:

```shell
ss -ltnp | grep -E '1143|1025'
```

## 2. Write the configuration

Create `~/.config/himalaya/config.toml`:

```toml
[accounts.proton]
default = true
email = "your-address@proton.me"
display-name = "your display name"

# IMAP — must be `login`, not `plain`.
imap.server = "imap://127.0.0.1:1143"
imap.sasl.login.username = "your-address"
imap.sasl.login.password.raw = "your bridge password"

# SMTP — must be `plain`, not `login`.
smtp.server = "smtp://127.0.0.1:1025"
smtp.sasl.plain.username = "your-address"
smtp.sasl.plain.password.raw = "your bridge password"

message.send.save-copy = "Sent"
```

The `username` is your ProtonMail username, and the password is the **bridge
password** printed by `monoxide auth <username>` — not your ProtonMail password.

Lock the file down, it holds that password:

```shell
chmod 600 ~/.config/himalaya/config.toml
```

## 3. Test in this order

Each step depends on the previous one, so stop at the first failure rather than
running them all at once.

```shell
himalaya account list        # 1. account detected, backends "imap, smtp"
himalaya mailbox list        # 2. your ProtonMail folders appear
himalaya envelope list -s 5  # 3. message subjects show up
himalaya message read 1      # 4. a body is decrypted and displayed
```

If step 1 prints an empty table, the TOML failed to parse — check
`himalaya account list` for a parse error. If step 2 fails on authentication,
you are on the wrong SASL mechanism; go back to the table above.

Because the account is marked `default = true`, `-a proton` is optional.

## 4. Sending

Himalaya v2 builds the message itself: recipients come from the `To:` header,
not from a `-t` flag. Write an RFC 5322 file and pass it after `--`:

```shell
cat > /tmp/mail.eml <<'EOF'
From: your-address@proton.me
To: someone@example.org
Subject: Hello

Body text.
EOF

himalaya message send -- /tmp/mail.eml
```

A successful send prints `Message successfully sent`. Copies land in the
mailbox named by `message.send.save-copy`.

The `From:` address must match one of your ProtonMail addresses exactly —
`monoxide` rejects anything else with `unknown sender address`.

## Running as a service

A bridge started by hand dies with your terminal. To keep it running across
reboots, use a systemd user unit.

Create `~/.config/systemd/user/monoxide.env` holding the password, mode `600`,
and keep it out of the unit file itself:

```shell
cat > ~/.config/systemd/user/monoxide.env <<'EOF'
HYDROXIDE_BRIDGE_PASS=your bridge password
EOF

chmod 600 ~/.config/systemd/user/monoxide.env
```

Then `~/.config/systemd/user/monoxide.service`:

```ini
[Unit]
Description=monoxide ProtonMail bridge
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%h/.config/systemd/user/monoxide.env
ExecStart=%h/.local/bin/monoxide serve
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
```

Enable it and start it:

```shell
systemctl --user daemon-reload
systemctl --user enable --now monoxide.service
systemctl --user status monoxide.service
```

### Surviving a reboot without a session

By default a user unit stops when you log out, so it will **not** come back after
a reboot until you log in again. Turn on lingering once:

```shell
sudo loginctl enable-linger "$USER"
```

After that the unit starts at boot with no login session required. Check with
`loginctl show-user "$USER" -p Linger`, which should read `Linger=yes`.

## Security

* Both servers are cleartext and bind to `127.0.0.1` only, so the password never
  crosses the network — but anything running as your user can read it, including
  the config file and the systemd env file.
* `monoxide serve` also opens CardDAV and CalDAV on 8080 and 8081. They speak
  cleartext HTTP, so a reverse proxy with TLS is required before any other
  machine can reach them. If you do not need them, start the two servers you do
  need instead:

  ```shell
  monoxide imap &
  monoxide smtp &
  ```

* Never expose 1025 or 1143 beyond localhost, and put an HTTPS reverse proxy in
  front of CardDAV and CalDAV before reaching them from another machine.
* Anyone with your bridge password can decrypt your stored ProtonMail
  credentials. Treat it like a password manager master key, and do not put it in
  a repository.