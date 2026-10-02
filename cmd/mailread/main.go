package main

// mailread lists and prints ProtonMail messages. Useful for reading mail
// through the bridge without a client.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Tahlasandale/monoxide/auth"
	"github.com/Tahlasandale/monoxide/protonmail"
)

func main() {
	username := flag.String("user", "", "ProtonMail username")
	limit := flag.Int("limit", 10, "max messages to list")
	read := flag.Bool("read", false, "print message bodies")
	markRead := flag.Bool("mark-read", false, "mark listed messages as read")
	all := flag.Bool("all", false, "include read messages")
	flag.Parse()

	if *username == "" {
		fmt.Fprintln(os.Stderr, "usage: mailread -user <username> [-read] [-mark-read]")
		os.Exit(2)
	}

	pass := os.Getenv("HYDROXIDE_BRIDGE_PASS")
	if pass == "" {
		fmt.Fprintln(os.Stderr, "HYDROXIDE_BRIDGE_PASS is not set")
		os.Exit(1)
	}

	m := auth.NewManager(func() *protonmail.Client {
		return &protonmail.Client{
			RootURL:    "https://mail.proton.me/api",
			AppVersion: "Other",
		}
	})

	c, privateKeys, err := m.Auth(*username, pass)
	if err != nil {
		fmt.Fprintln(os.Stderr, "auth failed:", err)
		os.Exit(1)
	}

	addrs, err := c.ListAddresses()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot list addresses:", err)
		os.Exit(1)
	}
	if len(addrs) == 0 {
		fmt.Fprintln(os.Stderr, "account has no address")
		os.Exit(1)
	}
	// Proton's /messages endpoint requires an AddressID.
	addressID := addrs[0].ID

	filter := &protonmail.MessageFilter{
		Label:     protonmail.LabelInbox,
		Sort:      "Time",
		Asc:       false,
		Page:      1,
		PageSize:  *limit,
		AddressID: addressID,
	}
	label := "message(s) in inbox"
	if !*all {
		unread := true
		filter.Unread = &unread
		label = "unread message(s) in inbox"
	}
	total, msgs, err := c.ListMessages(filter)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list failed:", err)
		os.Exit(1)
	}

	fmt.Printf("%d %s\n\n", total, label)

	var toMark []string
	for i, msg := range msgs {
		from := "?"
		if msg.Sender != nil {
			from = msg.Sender.Address
		}
		marker := " "
		if msg.Unread == 1 {
			marker = "*"
		}
		fmt.Printf("%s [%d] %s\n", marker, i+1, firstLine(msg.Subject))
		fmt.Printf("    de: %s\n", from)
		fmt.Printf("    %s\n", msg.Time.Time().Format("2006-01-02 15:04"))
		if msg.NumAttachments > 0 {
			fmt.Printf("    %d piece(s) jointe(s)\n", msg.NumAttachments)
		}
		if !*read {
			continue
		}

		md, err := msg.Read(privateKeys, nil)
		if err != nil {
			fmt.Printf("    [dechiffrement impossible: %v]\n", err)
			continue
		}
		body, _ := io.ReadAll(md.UnverifiedBody)
		fmt.Printf("    ---\n")
		for _, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
			fmt.Printf("    %s\n", line)
		}
		fmt.Printf("    ---\n")
		toMark = append(toMark, msg.ID)
	}

	if *markRead && len(toMark) > 0 {
		if err := c.MarkMessagesRead(toMark); err != nil {
			fmt.Fprintf(os.Stderr, "mark as read failed: %v\n", err)
		} else {
			fmt.Printf("\n%d message(s) marque(s) comme lus\n", len(toMark))
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		s = "(pas d'objet)"
	}
	return s
}
