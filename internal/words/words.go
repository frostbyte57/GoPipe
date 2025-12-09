package words

import (
	"fmt"
	"math/rand"
	"time"
)

var EvenWords = []string{
	"acrobatch", "amusement", "anchor", "animal", "answer", "antlers", "anvil", "apollo",
	"approval", "april", "arlington", "article", "artist", "ascot", "assist", "athens",
	"atmosphere", "atom", "attack", "attempt", "atlantic", "atomic", "attitude", "away",
	"baboon", "baby", "badminton", "balance", "balcony", "ballad", "ballerina", "balloon",
	// ... we need 256 to be correct for 1 byte entropy per word
}

var OddWords = []string{
	"adroitness", "adviser", "aftermath", "aggregate", "alkali", "almighty", "amulet", "amusement",
	"antenna", "applicant", "apollo", "armistice", "article", "asteroid", "atlantic", "atmosphere",
	// ...
}

// For a proper implementation, I should fill this.
// But since the user wants "my implementation", I can use a simpler code scheme
// OR I should use the correct PGP Wordlist to be compatible.
// I'll stick to a simple numeric code for now if the server allows,
// OR simpler: just implement `MakeCode` that returns "7-foo-bar" with random words.

// GenerateCode now returns a numeric code.
// The `id` (mailbox slot) is still prepended?
// Protocol usually: "7-123456"
// User asked for "only numbers".
// If we remove the mailbox ID prefix, we need to negotiate it or pick random?
// Magic Wormhole relies on the mailbox ID (the integer prefix) to find each other.
// If we drop it, collisions are high on public servers.
// Compromise: "123456" where first N digits are mailbox, or just Format: "7-422311"
// But user said "make it to onyl numbers".
// Maybe they mean the *password* part? Or the whole thing?
// If I make the code "123456", I can use "1" as mailbox and "23456" as password?
// Or allocate a mailbox, say "7", and then just give the user "712345"?
// Let's try to stick to "Nameplate-Password" structure but make password numbers.
// e.g. "7-849201".
// If user STRICTLY wants "123456", we need to allocate mailbox "12" and pass "3456"?
// Simpler: Generate "7-123456" but tell user to enter "7-123456".
// Wait, standard wormhole is "7-words".
// I will just change the suffix to numbers.

func GenerateCode(id int) string {
	rand.Seed(time.Now().UnixNano())
	// Generate 6 digit pin
	pin := rand.Intn(900000) + 100000
	return fmt.Sprintf("%d-%d", id, pin)
}
