package detect

import "strings"

// contractions maps English contractions to their expanded forms.
// Expanded forms prevent contraction fragments (e.g. "Don", "Ill") from
// leaking through as false positive name candidates.
var contractions = map[string]string{
	"don't":     "do not",
	"can't":     "cannot",
	"won't":     "will not",
	"isn't":     "is not",
	"aren't":    "are not",
	"wasn't":    "was not",
	"weren't":   "were not",
	"hasn't":    "has not",
	"haven't":   "have not",
	"hadn't":    "had not",
	"doesn't":   "does not",
	"didn't":    "did not",
	"couldn't":  "could not",
	"wouldn't":  "would not",
	"shouldn't": "should not",
	"mustn't":   "must not",
	"mightn't":  "might not",
	"needn't":   "need not",
	"daren't":   "dare not",
	"i'm":       "i am",
	"you're":    "you are",
	"we're":     "we are",
	"they're":   "they are",
	"he's":      "he is",
	"she's":     "she is",
	"it's":      "it is",
	"that's":    "that is",
	"what's":    "what is",
	"who's":     "who is",
	"there's":   "there is",
	"where's":   "where is",
	"how's":     "how is",
	"why's":     "why is",
	"i'll":      "i will",
	"you'll":    "you will",
	"we'll":     "we will",
	"they'll":   "they will",
	"he'll":     "he will",
	"she'll":    "she will",
	"it'll":     "it will",
	"that'll":   "that will",
	"i've":      "i have",
	"you've":    "you have",
	"we've":     "we have",
	"they've":   "they have",
	"i'd":       "i would",
	"you'd":     "you would",
	"we'd":      "we would",
	"they'd":    "they would",
	"he'd":      "he would",
	"she'd":     "she would",
	"let's":     "let us",
}

// englishWords is a set of common English words that should never be
// treated as character names. Every key is lowercase; lookup checks
// strings.ToLower(token). Contains a curated set of pronouns,
// common dialogue words, prepositions, and other high-frequency tokens.
var englishWords = map[string]bool{
	// pronouns
	"i": true, "you": true, "he": true, "she": true, "it": true,
	"we": true, "they": true, "me": true, "him": true, "her": true,
	"us": true, "them": true, "my": true, "your": true, "his": true,
	"its": true, "our": true, "their": true, "mine": true, "yours": true,
	"hers": true, "ours": true, "theirs": true, "myself": true,
	"yourself": true, "himself": true, "herself": true, "itself": true,
	"ourselves": true, "yourselves": true, "themselves": true,

	// common dialogue words
	"yeah": true, "yep": true, "yup": true, "no": true, "nope": true,
	"yes": true, "okay": true, "ok": true, "oh": true, "ah": true,
	"uh": true, "ugh": true, "huh": true, "hey": true, "wow": true,
	"ooh": true, "aah": true, "ouch": true, "whoa": true, "hmm": true,
	"umm": true, "um": true, "er": true, "well": true, "so": true,
	"now": true, "then": true, "anyway": true, "anyways": true,
	"actually": true, "honestly": true, "basically": true,
	"seriously": true, "literally": true, "absolutely": true,
	"definitely": true, "probably": true, "maybe": true, "perhaps": true,

	// question words
	"what": true, "why": true, "how": true, "when": true, "where": true,
	"who": true, "which": true, "whom": true, "whose": true,

	// articles and determiners
	"the": true, "a": true, "an": true, "this": true, "that": true,
	"these": true, "those": true, "some": true, "any": true,
	"every": true, "each": true, "all": true, "none": true,
	"both": true, "either": true, "neither": true, "few": true,
	"many": true, "several": true, "such": true, "other": true,
	// prepositions
	"in": true, "on": true, "at": true, "to": true, "for": true,
	"with": true, "by": true, "from": true, "about": true,
	"into": true, "through": true, "during": true, "before": true,
	"after": true, "above": true, "below": true, "between": true,
	"under": true, "over": true, "without": true, "within": true,
	"along": true, "among": true, "behind": true,
	"beyond": true, "down": true, "off": true, "up": true, "out": true,
	"across": true, "against": true, "near": true,
	"until": true, "upon": true,

	// conjunctions
	"and": true, "but": true, "or": true, "nor": true,
	"because": true, "although": true, "though": true, "while": true,
	"if": true, "unless": true, "whereas": true,
	"whether": true, "till": true, "thus": true, "hence": true,
	"therefore": true,

	// common verbs
	"be": true, "am": true, "are": true, "is": true, "was": true,
	"were": true, "been": true, "being": true, "have": true, "has": true,
	"had": true, "having": true, "do": true, "does": true, "did": true,
	"doing": true, "done": true, "make": true, "made": true, "making": true,
	"take": true, "took": true, "taking": true, "given": true, "give": true,
	"gave": true, "get": true, "got": true, "getting": true, "go": true,
	"went": true, "going": true, "gone": true, "come": true, "came": true,
	"coming": true, "know": true, "knew": true, "known": true,
	"think": true, "thought": true, "see": true, "saw": true, "seen": true,
	"say": true, "tell": true, "told": true, "want": true,
	"wanted": true, "need": true, "needed": true, "find": true,
	"found": true, "keep": true, "kept": true, "put": true, "set": true,
	"let": true,
	"try": true, "tried": true, "show": true, "shown": true,
	"feel": true, "felt": true, "leave": true, "left": true,
	"mean": true, "meant": true, "become": true, "became": true,
	"begin": true, "began": true, "bring": true, "brought": true,
	"hold": true, "held": true, "look": true, "looked": true,
	"run": true, "ran": true, "stand": true, "stood": true,
	"turn": true, "turned": true, "hear": true, "heard": true,
	"help": true, "helped": true, "hate": true, "hated": true,
	"hope": true, "hoped": true, "wish": true, "wished": true,
	"guess": true, "guessed": true, "wonder": true, "wondered": true,
	"remember": true, "remembered": true,
	"forget": true, "forgot": true, "forgiven": true, "sorry": true,
	"miss": true, "missed": true, "not": true, "friends": true, "speak": true, "spoke": true,
	"spoken": true, "talk": true, "talked": true, "walk": true,
	"walked": true, "wait": true, "waited": true, "stop": true,
	"stopped": true, "start": true, "started": true, "stay": true,
	"stayed": true, "live": true, "lived": true, "work": true,
	"worked": true, "play": true, "played": true, "eat": true,
	"ate": true, "drink": true, "drank": true,
	"sleep": true, "slept": true, "read": true, "write": true,
	"wrote": true, "listen": true, "listened": true,
	"love": true, "liked": true,

	// modal verbs
	"can": true, "could": true, "will": true, "would": true,
	"shall": true, "should": true, "may": true, "might": true,
	"must": true,

	// common adjectives
	"good": true, "better": true, "best": true, "bad": true,
	"worse": true, "worst": true, "fine": true, "great": true,
	"nice": true, "kind": true, "sure": true, "right": true,
	"wrong": true, "true": true, "false": true, "real": true,
	"new": true, "old": true, "big": true, "small": true,
	"large": true, "little": true, "high": true, "low": true,
	"long": true, "short": true, "fast": true, "slow": true,
	"hard": true, "easy": true, "happy": true, "sad": true,
	"mad": true, "glad": true, "tired": true, "pretty": true,
	"cute": true, "hot": true, "cold": true, "warm": true,
	"cool": true, "early": true, "late": true, "next": true,
	"last": true, "first": true, "second": true, "third": true,
	"only": true, "different": true, "special": true,
	"normal": true, "ordinary": true, "strange": true, "simple": true,
	"clear": true, "full": true, "empty": true, "open": true,
	"closed": true, "free": true, "busy": true, "ready": true,
	"alone": true, "together": true, "fair": true, "worth": true,
	"close": true, "deep": true, "wide": true,
	"extra": true, "super": true, "ultra": true, "mega": true,

	// time and frequency
	"today": true, "tomorrow": true, "yesterday": true, "morning": true,
	"afternoon": true, "evening": true, "night": true, "midnight": true,
	"noon": true, "always": true, "never": true, "often": true,
	"sometimes": true, "usually": true, "rarely": true,
	"occasionally": true, "frequently": true, "constantly": true,
	"already": true, "still": true, "just": true,
	"recently": true, "earlier": true, "later": true, "soon": true,
	"immediately": true, "finally": true, "eventually": true,
	"currently": true, "previously": true, "initially": true,
	"meanwhile": true,

	// place / direction
	"here": true, "there": true, "everywhere": true, "anywhere": true,
	"somewhere": true, "nowhere": true, "inside": true, "outside": true,
	"upstairs": true, "downstairs": true, "back": true, "forward": true,
	"backward": true, "away": true,
	"home": true,

	// misc dialogue / filler
	"hello": true, "hi": true, "bye": true, "goodbye": true,
	"please": true, "thanks": true, "thank": true, "excuse": true,
	"pardon": true, "welcome": true, "oops": true, "whoops": true,
	"darn": true, "gosh": true, "gee": true, "man": true, "dude": true,
	"bro": true, "sis": true, "friend": true, "buddy": true,
	"pal": true, "mate": true, "guys": true, "folks": true,
	"everyone": true, "everybody": true, "someone": true,
	"somebody": true, "anyone": true, "anybody": true, "noone": true,
	"nobody": true, "everything": true, "nothing": true, "something": true,
	"anything": true, "enough": true, "less": true,
	"least": true, "plenty": true, "lot": true, "lots": true, "introvert": true,

	// number words
	"one": true, "two": true, "three": true, "four": true, "five": true,
	"six": true, "seven": true, "eight": true, "nine": true, "ten": true,
	"hundred": true, "thousand": true, "million": true, "billion": true,

	// ordinal / common narrative words
	"also": true, "even": true,
	"too": true, "very": true, "quite": true, "rather": true,
	"extremely": true, "incredibly": true,
	"especially": true, "particularly": true, "supposed": true,
	"certain": true, "certainly": true, "obviously": true,
	"apparently": true, "naturally": true, "exactly": true,
	"precisely": true, "indeed": true,
	"besides": true, "furthermore": true,
	"nevertheless": true, "nonetheless": true,

	// verbs of speech (common in dialogue)
	"says": true,
	"asks": true, "reply": true, "replied": true, "replies": true,
	"answer": true, "answered": true, "answers": true,
	"shout": true, "shouted": true, "shouts": true,
	"whisper": true, "whispered": true, "whispers": true,
	"mutter": true, "muttered": true, "mutters": true,
	"yell": true, "yelled": true, "yells": true,
	"scream": true, "screamed": true, "screams": true,
	"calls":   true,
	"exclaim": true, "exclaimed": true, "exclaims": true,
	"mention": true, "mentioned": true, "mentions": true,
	"respond": true, "responded": true, "responds": true,
	"continue": true, "continued": true, "continues": true,
	"add": true, "added": true, "adds": true,
	"note": true, "noted": true, "notes": true,
}

// englishSuffixes contains morphological endings that strongly indicate
// an English word rather than a Japanese name.
var englishSuffixes = []string{
	"tion",
	"sion",
	"ness",
	"ment",
	"cious",
	"tious",
	"able",
	"ible",
}

// hasEnglishSuffix returns true if the word ends with a known English suffix.
func hasEnglishSuffix(s string) bool {
	lower := strings.ToLower(s)
	for _, suf := range englishSuffixes {
		if strings.HasSuffix(lower, suf) && len(s) >= len(suf)+3 {
			return true
		}
	}
	// Reject -ing for longer words (gerunds / present participles).
	if strings.HasSuffix(lower, "ing") && len(s) >= 6 {
		return true
	}
	return false
}

// aliases maps known name variants to their canonical form.
// Handles nicknames, truncated versions, and alternative spellings.
var aliases = map[string]string{
	"Rena":    "Renako",
	"Satsuki": "Satuki",
	"Satu":    "Satuki",
	"Ajis":    "Ajisai",
	"Aji":     "Ajisai",
}

// isEnglishWord reports whether a token is a known English word.
func isEnglishWord(s string) bool {
	return englishWords[strings.ToLower(s)]
}
