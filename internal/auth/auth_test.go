package auth_test

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lclpyt/ledger/internal/auth"
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Suite")
}

var _ = Describe("Token", func() {
	It("parses a token it created", func() {
		token, id, hash, err := auth.NewToken()
		Expect(err).ToNot(HaveOccurred())

		parsedID, secret, ok := auth.ParseToken(token)

		Expect(ok).To(BeTrue())
		Expect(parsedID).To(Equal(id))
		Expect(auth.HashSecret(secret)).To(Equal(hash))
	})

	It("creates a distinct token every time", func() {
		first, _, _, err := auth.NewToken()
		Expect(err).ToNot(HaveOccurred())

		second, _, _, err := auth.NewToken()
		Expect(err).ToNot(HaveOccurred())

		Expect(first).ToNot(Equal(second))
	})

	DescribeTable("rejects a malformed token",
		func(mutate func(string) string) {
			token, _, _, err := auth.NewToken()
			Expect(err).ToNot(HaveOccurred())

			_, _, ok := auth.ParseToken(mutate(token))

			Expect(ok).To(BeFalse())
		},
		Entry("empty", func(string) string { return "" }),
		Entry("wrong prefix", func(t string) string { return "xyz_" + t[4:] }),
		Entry("truncated", func(t string) string { return t[:len(t)-1] }),
		Entry("extended", func(t string) string { return t + "a" }),
		Entry("missing separator", func(t string) string { return t[:36] + "-" + t[37:] }),
		Entry("non hex id", func(t string) string { return t[:4] + "zz" + t[6:] }),
	)
})

var _ = Describe("Permissions", func() {
	DescribeTable("Grants",
		func(granted, required string, expected bool) {
			Expect(auth.Grants(granted, required)).To(Equal(expected))
		},
		Entry("exact match", "games.read", "games.read", true),
		Entry("different action", "games.read", "games.write", false),
		Entry("global wildcard", "*", "assets.read:maps", true),
		Entry("action wildcard", "games.*", "games.write", true),
		Entry("action wildcard covers scopes", "assets.*", "assets.read:maps", true),
		Entry("action wildcard of another resource", "games.*", "player_settings.read", false),
		Entry("scope wildcard", "assets.read:*", "assets.read:maps", true),
		Entry("scope wildcard of another action", "assets.read:*", "assets.write:maps", false),
		Entry("scope", "assets.read:maps", "assets.read:maps", true),
		Entry("other scope", "assets.read:maps", "assets.read:skins", false),
		Entry("resource prefix is no wildcard", "games", "games.read", false),
	)

	DescribeTable("ValidPermission",
		func(permission string, expected bool) {
			Expect(auth.ValidPermission(permission)).To(Equal(expected))
		},
		Entry("global wildcard", "*", true),
		Entry("action", "games.read", true),
		Entry("action wildcard", "games.*", true),
		Entry("scope", "assets.read:maps-2", true),
		Entry("scope wildcard", "assets.read:*", true),
		Entry("empty", "", false),
		Entry("upper case", "Games.read", false),
		Entry("wildcard in the middle", "games.*.read", false),
		Entry("partial wildcard", "games.re*", false),
		Entry("too long", strings.Repeat("a", 256), false),
	)
})

var _ = Describe("Principal", func() {
	It("can do what any of its permissions grants", func() {
		p := &auth.Principal{Permissions: []string{"games.read", "assets.read:*"}}

		Expect(p.Can("games.read")).To(BeTrue())
		Expect(p.Can("assets.read:maps")).To(BeTrue())
		Expect(p.Can("games.write")).To(BeFalse())
	})

	It("can do nothing without permissions", func() {
		Expect((&auth.Principal{}).Can("games.read")).To(BeFalse())
	})

	It("can only do what its scopes cover", func() {
		p := &auth.Principal{Permissions: []string{"*"}, Scopes: []string{"games.read", "assets.*"}}

		Expect(p.Can("games.read")).To(BeTrue())
		Expect(p.Can("assets.write:maps")).To(BeTrue())
		Expect(p.Can("games.write")).To(BeFalse())
	})

	It("cannot do what its scopes cover but its permissions do not", func() {
		p := &auth.Principal{Permissions: []string{"assets.read:*"}, Scopes: []string{"assets.*"}}

		Expect(p.Can("assets.read:maps")).To(BeTrue())
		Expect(p.Can("assets.write:maps")).To(BeFalse())
	})
})
