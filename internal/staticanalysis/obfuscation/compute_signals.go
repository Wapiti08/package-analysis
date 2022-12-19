package obfuscation

import (
	"math"
	"regexp"
	"strings"

	"github.com/ossf/package-analysis/internal/staticanalysis/obfuscation/stats"
	"github.com/ossf/package-analysis/internal/staticanalysis/obfuscation/stringentropy"
	"github.com/ossf/package-analysis/internal/staticanalysis/token"
	"github.com/ossf/package-analysis/internal/utils"
)

var suspiciousIdentifierPatterns = map[string]*regexp.Regexp{
	"hex":     regexp.MustCompile("^_0x\\d{3,}$"),
	"numeric": regexp.MustCompile("^[A-Za-z_]?\\d{3,}$"),
}

// Adapted from https://stackoverflow.com/a/5885097 to only match
// base64 strings with at least 12 characters
var longBase64String = regexp.MustCompile("(?:[A-Za-z0-9+/]{4}){3,}(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{4})")

// to avoid false positive matching on long words, hex strings or file paths, etc.
// check for at least one digit and one letter outside a-f
var digit = regexp.MustCompile("\\d")
var nonHexLetter = regexp.MustCompile("[G-Zg-z]")

/*
characterAnalysis performs analysis on a collection of string symbols, returning:
- Counts of symbol (string) lengths
- Stats summary of symbol (string) entropies
- Entropy of all symbols concatenated together
*/
func characterAnalysis(symbols []string) (
	lengthCounts map[int]int,
	entropySummary stats.SampleStatistics,
	combinedEntropy float64,
) {
	// measure character probabilities by looking at entire set of strings
	characterProbs := stringentropy.CharacterProbabilities(symbols)

	var entropies []float64
	var lengths []int
	for _, s := range symbols {
		entropies = append(entropies, stringentropy.CalculateEntropy(s, characterProbs))
		lengths = append(lengths, len(s))
	}

	lengthCounts = stats.CountDistinct(lengths)
	entropySummary = stats.Summarise(entropies)
	combinedEntropy = stringentropy.CalculateEntropy(strings.Join(symbols, ""), nil)
	return
}

/*
ComputeSignals creates a FileSignals object based on the data obtained from CollectData
for a given file. These signals may be useful to determine whether the code is obfuscated.
*/
func ComputeSignals(rawData FileData) FileSignals {
	signals := FileSignals{}

	literals := utils.Transform(rawData.StringLiterals, func(s token.String) string { return s.Value })
	signals.StringLengths, signals.StringEntropySummary, signals.CombinedStringEntropy =
		characterAnalysis(literals)

	identifierNames := utils.Transform(rawData.Identifiers, func(i token.Identifier) string { return i.Name })
	signals.IdentifierLengths, signals.IdentifierEntropySummary, signals.CombinedIdentifierEntropy =
		characterAnalysis(identifierNames)

	signals.SuspiciousIdentifiers = map[string][]string{}
	for ruleName, pattern := range suspiciousIdentifierPatterns {
		signals.SuspiciousIdentifiers[ruleName] = []string{}
		for _, name := range identifierNames {
			if pattern.MatchString(name) {
				signals.SuspiciousIdentifiers[ruleName] = append(signals.SuspiciousIdentifiers[ruleName], name)
			}
		}
	}

	signals.Base64Strings = []string{}
	for _, s := range literals {
		matches := longBase64String.FindAllString(s, -1)
		for _, candidate := range matches {
			// use some extra checks to reduce false positives
			if digit.MatchString(candidate) && nonHexLetter.MatchString(candidate) {
				signals.Base64Strings = append(signals.Base64Strings, matches...)
			}
		}
	}

	return signals
}

func NoSignals() FileSignals {
	return FileSignals{
		StringLengths:             map[int]int{},
		StringEntropySummary:      stats.NoData(),
		CombinedStringEntropy:     math.NaN(),
		IdentifierLengths:         map[int]int{},
		IdentifierEntropySummary:  stats.NoData(),
		CombinedIdentifierEntropy: math.NaN(),
	}
}

// RemoveNaNs replaces all NaN values in this object with zeros
func RemoveNaNs(s *FileSignals) {
	s.StringEntropySummary = s.StringEntropySummary.ReplaceNaNs(0)
	s.IdentifierEntropySummary = s.IdentifierEntropySummary.ReplaceNaNs(0)

	if math.IsNaN(s.CombinedStringEntropy) {
		s.CombinedStringEntropy = 0.0
	}
	if math.IsNaN(s.CombinedIdentifierEntropy) {
		s.CombinedIdentifierEntropy = 0.0
	}
}
