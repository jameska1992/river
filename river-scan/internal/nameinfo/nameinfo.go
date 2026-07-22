// Package nameinfo extracts a clean title, year, and external IDs from movie
// file or directory names, and from Kodi/Jellyfin/Plex NFO sidecars.
//
// It is intentionally movie-focused: aggressive token stripping is safe for
// movies but would corrupt music album or TV episode titles, so callers
// should only use it on the movie path.
package nameinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Info is the result of parsing a name. Zero values mean "not found".
type Info struct {
	Title  string
	Year   int
	TMDBID int    // 0 if not present
	IMDBID string // e.g. "tt0133093", lowercase, empty if not present
}

// Plex/Jellyfin/Kodi embed external IDs in bracketed tags:
//
//	{tmdb-603}, [tmdbid-603], {tmdb=603}
//	{imdb-tt0133093}, [imdbid-tt0133093]
var (
	tmdbIDRe   = regexp.MustCompile(`(?i)[\[{(]\s*tmdb(?:id)?\s*[-=:\s]+(\d+)\s*[\]})]`)
	imdbIDRe   = regexp.MustCompile(`(?i)[\[{(]\s*imdb(?:id)?\s*[-=:\s]+(tt\d{6,10})\s*[\]})]`)
	bareIMDBRe = regexp.MustCompile(`\btt\d{6,10}\b`)

	// curly/square brackets and their contents — stripped after ID extraction.
	bracketedRe = regexp.MustCompile(`[\[{][^\]}]*[\]}]`)

	// 4-digit year in 1900..2099 (range further restricted at parse time).
	yearRe = regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)

	// release separators normalized to spaces (run after bracket stripping).
	separatorRe = regexp.MustCompile(`[._\-()]+`)
)

// boundaryTokens are unambiguous release-tag tokens that never appear in
// real movie titles. When one is encountered we slice the token list at
// that position — everything after it is release-name noise (including
// release-group names like "Pimp4003" or "FraMeSToR" that aren't in any
// static list). This is a subset of noiseTokens.
var boundaryTokens = map[string]bool{
	// resolution
	"480p": true, "576p": true, "720p": true, "1080p": true, "1440p": true,
	"2160p": true, "4k": true, "8k": true, "uhd": true, "fhd": true,
	// source
	"bluray": true, "blu-ray": true, "bdrip": true, "brrip": true,
	"bdremux": true, "remux": true, "webrip": true, "web-dl": true, "webdl": true,
	"hdrip": true, "dvdrip": true, "dvdr": true,
	"hdtv": true, "pdtv": true, "sdtv": true, "hdcam": true,
	"telesync": true, "telecine": true, "r5": true,
	// codec
	"x264": true, "x265": true, "h264": true, "h265": true, "h.264": true,
	"h.265": true, "hevc": true, "avc": true, "xvid": true, "divx": true,
	"vp9": true, "av1": true, "mpeg2": true, "mpeg4": true,
}

// noiseTokens are the release-name tags routinely tacked onto movie filenames.
// Tokens are matched case-insensitively after separator normalization, so
// "1080p", "BluRay", "x264", "DD5.1", etc. all get dropped. This is the
// superset of boundaryTokens — boundary tokens slice, the rest just filter.
var noiseTokens = map[string]bool{
	// resolution
	"480p": true, "576p": true, "720p": true, "1080p": true, "1440p": true,
	"2160p": true, "4k": true, "8k": true, "uhd": true, "fhd": true, "hd": true,
	// source
	"bluray": true, "blu-ray": true, "bdrip": true, "brrip": true, "bd": true,
	"bdremux": true, "remux": true, "webrip": true, "web-dl": true, "webdl": true,
	"web": true, "hdrip": true, "dvdrip": true, "dvd": true, "dvdr": true,
	"hdtv": true, "pdtv": true, "sdtv": true, "cam": true, "ts": true, "tc": true,
	"r5": true, "hc": true, "hdcam": true, "telesync": true, "telecine": true,
	// codec
	"x264": true, "x265": true, "h264": true, "h265": true, "h.264": true,
	"h.265": true, "hevc": true, "avc": true, "xvid": true, "divx": true,
	"vp9": true, "av1": true, "mpeg2": true, "mpeg4": true,
	// audio
	"dts": true, "dts-hd": true, "dtshd": true, "dts-x": true, "dtsx": true,
	"ac3": true, "eac3": true, "ddp": true, "ddp5.1": true, "dd5.1": true,
	"dd+": true, "ddplus": true, "aac": true, "aac2.0": true, "mp3": true,
	"flac": true, "atmos": true, "truehd": true, "ma": true, "opus": true,
	// channels
	"2.0": true, "5.1": true, "7.1": true,
	// HDR / color
	"hdr": true, "hdr10": true, "hdr10+": true, "dv": true, "dolbyvision": true, "sdr": true,
	// editions / flags
	"extended": true, "unrated": true, "uncut": true, "theatrical": true,
	"directors": true, "director's": true, "imax": true, "remastered": true,
	"remaster": true, "anniversary": true, "criterion": true, "proper": true,
	"repack": true, "rerip": true, "internal": true, "limited": true,
	"multi": true, "dual": true, "readnfo": true, "subbed": true, "dubbed": true,
}

// Parse extracts title/year/IDs from a single name (directory or filename
// without extension). It is safe to call on either form.
func Parse(name string) Info {
	if name == "" {
		return Info{}
	}
	var info Info

	if m := tmdbIDRe.FindStringSubmatch(name); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			info.TMDBID = n
		}
	}
	if m := imdbIDRe.FindStringSubmatch(name); m != nil {
		info.IMDBID = strings.ToLower(m[1])
	}

	// Strip bracketed tags entirely now that IDs have been pulled out.
	cleaned := bracketedRe.ReplaceAllString(name, " ")

	// Catch bare "ttNNNNNNN" outside brackets as a last resort.
	if info.IMDBID == "" {
		if m := bareIMDBRe.FindString(cleaned); m != "" {
			info.IMDBID = strings.ToLower(m)
			cleaned = strings.Replace(cleaned, m, " ", 1)
		}
	}

	// Normalize "." "_" "-" "(" ")" runs into single spaces. Done before
	// findYear because Go's \b treats "_" as a word character; "Foo_2010_"
	// wouldn't match \b2010\b without the underscores becoming whitespace.
	cleaned = separatorRe.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	// Pick the right-most plausible year from the normalized string.
	// Fallback to the ORIGINAL name (with brackets) to catch bracketed
	// years like "[1962] 007 Dr No" that the bracket-strip above removed.
	if y, _ := findYear(cleaned); y > 0 {
		info.Year = y
	} else if y, _ := findYear(name); y > 0 {
		info.Year = y
	}

	// Slice at the first release-tag boundary token (resolution / source /
	// codec). Everything after a boundary is release noise — including
	// release-group names ("Pimp4003", "FraMeSToR") that aren't in any
	// static list. This catches them for free.
	tokens := strings.Fields(cleaned)
	for i, t := range tokens {
		if boundaryTokens[strings.ToLower(strings.Trim(t, "-_."))] {
			tokens = tokens[:i]
			break
		}
	}

	// Drop decoration noise + the year token itself. Filtering the year
	// here — instead of slicing the cleaned string at its position — lets
	// us keep content that surrounds the year ("2001 A Space Odyssey 1968"
	// → "2001 A Space Odyssey") and handle bracketed-year prefixes
	// ("[1962] 007 Dr No" → "007 Dr No") in the same code path.
	yearStr := ""
	if info.Year > 0 {
		yearStr = strconv.Itoa(info.Year)
	}
	kept := tokens[:0]
	for _, t := range tokens {
		key := strings.ToLower(strings.Trim(t, "-_."))
		if key == "" || noiseTokens[key] || (yearStr != "" && key == yearStr) {
			continue
		}
		kept = append(kept, t)
	}
	info.Title = strings.TrimSpace(strings.Join(kept, " "))
	return info
}

// findYear returns the rightmost 4-digit year in s that:
//   - lies in [1900, currentYear+1] (filters resolution-like numbers e.g. 2160), and
//   - is not at position 0 (titles like "2001 A Space Odyssey" keep their leading year).
func findYear(s string) (year, pos int) {
	matches := yearRe.FindAllStringIndex(s, -1)
	maxYear := time.Now().Year() + 1
	for i := len(matches) - 1; i >= 0; i-- {
		loc := matches[i]
		if loc[0] == 0 {
			continue
		}
		y, _ := strconv.Atoi(s[loc[0]:loc[1]])
		if y < 1900 || y > maxYear {
			continue
		}
		return y, loc[0]
	}
	return 0, -1
}

// ParseDir merges Parse(dirName) with hints from the largest video file's
// basename and any NFO sidecar found inside dir. NFO IDs take precedence,
// then in-name IDs, then filename IDs. Title and year prefer the directory
// name but fall back to the filename when missing.
func ParseDir(dir, dirName string, files []string) Info {
	info := Parse(dirName)

	if best := largestVideoFile(files); best != "" {
		base := strings.TrimSuffix(filepath.Base(best), filepath.Ext(best))
		sub := Parse(base)
		// A year-bearing filename outweighs a yearless directory: when the
		// directory is just a flat container ("Movies", "Untitled") but the
		// file is properly named, the file wins title+year.
		if info.Year == 0 && sub.Year != 0 {
			info.Title = sub.Title
			info.Year = sub.Year
		} else if info.Title == "" {
			info.Title = sub.Title
		}
		if info.TMDBID == 0 {
			info.TMDBID = sub.TMDBID
		}
		if info.IMDBID == "" {
			info.IMDBID = sub.IMDBID
		}
	}

	if tmdbID, imdbID, ok := ReadNFO(dir); ok {
		if tmdbID != 0 {
			info.TMDBID = tmdbID
		}
		if imdbID != "" {
			info.IMDBID = imdbID
		}
	}

	if info.Title == "" {
		info.Title = strings.TrimSpace(dirName)
	}
	return info
}

// NFO sidecars come in several flavors. Cover the common ones plus a bare
// "ttNNNNNNN" fallback (some scene NFOs are plain text with just the ID).
var (
	nfoIMDBRe       = regexp.MustCompile(`(?i)<imdb_?id>\s*(tt\d{6,10})\s*</imdb_?id>`)
	nfoTMDBRe       = regexp.MustCompile(`(?i)<tmdb_?id>\s*(\d+)\s*</tmdb_?id>`)
	nfoUniqueImdbRe = regexp.MustCompile(`(?i)<uniqueid[^>]*type=["']imdb["'][^>]*>\s*(tt\d{6,10})\s*</uniqueid>`)
	nfoUniqueTmdbRe = regexp.MustCompile(`(?i)<uniqueid[^>]*type=["']tmdb["'][^>]*>\s*(\d+)\s*</uniqueid>`)
)

// ReadNFO scans dir for movie.nfo or any *.nfo and returns the first
// IMDb/TMDB IDs it finds. ok=true if either ID was extracted.
func ReadNFO(dir string) (tmdbID int, imdbID string, ok bool) {
	candidates := nfoCandidates(dir)
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := string(data)

		var foundTMDB int
		var foundIMDB string
		if m := nfoIMDBRe.FindStringSubmatch(s); m != nil {
			foundIMDB = strings.ToLower(m[1])
		} else if m := nfoUniqueImdbRe.FindStringSubmatch(s); m != nil {
			foundIMDB = strings.ToLower(m[1])
		}
		if m := nfoTMDBRe.FindStringSubmatch(s); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n > 0 {
				foundTMDB = n
			}
		} else if m := nfoUniqueTmdbRe.FindStringSubmatch(s); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n > 0 {
				foundTMDB = n
			}
		}
		if foundIMDB == "" {
			if m := bareIMDBRe.FindString(s); m != "" {
				foundIMDB = strings.ToLower(m)
			}
		}
		if foundIMDB != "" || foundTMDB != 0 {
			return foundTMDB, foundIMDB, true
		}
	}
	return 0, "", false
}

// nfoCandidates lists NFO files inside dir, with movie.nfo prioritized.
func nfoCandidates(dir string) []string {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var preferred, rest []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.EqualFold(filepath.Ext(name), ".nfo") {
			continue
		}
		full := filepath.Join(dir, name)
		if strings.EqualFold(name, "movie.nfo") {
			preferred = append(preferred, full)
		} else {
			rest = append(rest, full)
		}
	}
	return append(preferred, rest...)
}

// remakeRe matches "remake" as a standalone word, case-insensitive.
// Used by RetryTitles to strip an artifact that's common in user-curated
// libraries when a recent version of a classic is on disk.
var remakeRe = regexp.MustCompile(`(?i)\bremake\b`)

// trailingOnesRe matches one or more " 1" tokens at the end of a string,
// e.g. "Toy Story 1" or "Show 1 1". The leading whitespace is required so we
// don't strip the "1" off "1917".
var trailingOnesRe = regexp.MustCompile(`(\s+1)+\s*$`)

// emptyBracketsRe catches the "(  )" / "[  ]" leftovers from stripping a
// parenthesized "remake" tag like "Suspiria (Remake)" → "Suspiria ( )".
var emptyBracketsRe = regexp.MustCompile(`[\(\[]\s*[\)\]]`)

// leadingNumberRe matches 1-4 leading digits followed by whitespace —
// handles collection prefixes like "007 Dr No" → "Dr No". The trailing
// whitespace requirement keeps it from eating numeric-only titles like
// "1917" or "300".
var leadingNumberRe = regexp.MustCompile(`^\d{1,4}\s+`)

// RetryTitles returns alternative search titles to try when the primary
// TMDB/etc. lookup fails. It does NOT include the original title — callers
// are expected to have already tried that. Variants are returned in
// "preserves the most" → "strips the most" order so the safest match is
// attempted first.
//
// Three transforms are applied, individually and in combination:
//
//   - "remake" (standalone word, case-insensitive) removed
//   - trailing " 1" runs removed
//   - leading 1-4 digit collection number removed ("007 Dr No" → "Dr No")
//
// These mirror common library-curation artifacts: remake suffixes,
// integer suffixes added to first installments after a sequel exists
// ("Toy Story 1"), and collection-index prefixes scrapers attach to
// franchise titles. Variants whose normalized form duplicates the
// original are skipped, so a title without artifacts produces an empty
// slice.
func RetryTitles(title string) []string {
	original := normalizeForCompare(title)
	if original == "" {
		return nil
	}

	stripRemake  := cleanTitle(remakeRe.ReplaceAllString(title, ""))
	stripOnes    := cleanTitle(trailingOnesRe.ReplaceAllString(title, ""))
	stripBoth    := cleanTitle(trailingOnesRe.ReplaceAllString(
		remakeRe.ReplaceAllString(title, ""), ""))
	stripLeading := cleanTitle(leadingNumberRe.ReplaceAllString(title, ""))
	stripAll     := cleanTitle(leadingNumberRe.ReplaceAllString(stripBoth, ""))

	var out []string
	seen := map[string]bool{original: true}
	for _, v := range []string{stripRemake, stripOnes, stripLeading, stripBoth, stripAll} {
		key := normalizeForCompare(v)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}

// cleanTitle tidies a variant after the regex strips run: removes empty
// "()"/"[]" left behind, collapses whitespace, trims edges.
func cleanTitle(s string) string {
	s = emptyBracketsRe.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

// normalizeForCompare is used only to dedupe variants — lowercase + trim is
// enough; we don't care about character classes here.
func normalizeForCompare(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// largestVideoFile returns the largest file in files by os.Stat size.
// Audio/subtitle extensions are tolerated — we just want a hint for the
// title parser, and the largest entry is almost always the movie file.
func largestVideoFile(files []string) string {
	var best string
	var bestSize int64
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			continue
		}
		if fi.Size() > bestSize {
			bestSize = fi.Size()
			best = f
		}
	}
	return best
}
