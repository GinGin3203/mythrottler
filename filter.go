package mythrottler

import (
	"net/http"
	"regexp"
	"strings"
)

/*
 shouldWait решает, будет ли запрос троттлиться или пройдет "вне очереди".
 Чтобы запрос троттлился, нужно чтобы:
 1) r.URL.Path не матчится регекспом tw.ignoredUrlsRegexp
 И
 2) r.Method найден в tw.httpMethods или tw.httpMethods пуст или nil
 И
 3) r.URL.Path матчится регекспом tw.urlsRegexp

 Во всех остальных случаях запрос не троттлится
*/
func (t *ThrottlingWrapper) shouldWait(r *http.Request) bool {
	if _, prs := t.httpMethods[r.Method]; !t.
		ignoredPathPrefixRegexp.MatchString(r.URL.Path) &&
		(prs || len(t.httpMethods) == 0) &&
		t.pathPrefixRegexp.MatchString(r.URL.Path) {
		return true
	}

	return false
}

// prefixMatcher возвращает регексп, матчащий префиксы путей, запросы по которыми будут троттлиться.
// Если urlPrefixes пуст или nil, регексп матчит все пути
func prefixMatcher(urlPrefixes []string) (prefPattern *regexp.Regexp, err error) {
	if prefPattern, err = regexp.Compile(prefixMatcherRaw(urlPrefixes)); err != nil {
		return nil, err
	}
	return prefPattern, nil
}

// prefixMatcherIgnored возвращает регексп, матчащий префиксы путей, запросы по которым не будут троттлиться.
// Если urlPrefixesIgnored пустой или nil, регексп не матчит никакой путь
func prefixMatcherIgnored(urlPrefixesIgnored []string) (ignoredPrefPattern *regexp.Regexp, err error) {
	var raw string
	if raw = prefixMatcherRaw(urlPrefixesIgnored); raw == "" {
		// url.URL.Path может либо быть пустым, либо начинаться с '/'.
		// Регексп '^[^/ ]' ничего не заматчит в пустой строке или в строке, начинающейся с '/',
		// поэтому он не заматчит ни одного пути
		raw = "^[^/]"
	}

	if ignoredPrefPattern, err = regexp.Compile(raw); err != nil {
		return nil, err
	}

	return ignoredPrefPattern, nil
}

func prefixMatcherRaw(urlPrefixes []string) (rawRegexp string) {
	for i, pref := range urlPrefixes {
		if pref == "" {
			continue
		}

		rawRegexp +=
			// хотим матчить префиксы путей, поэтому приклеиваем '^' в начало паттерна
			"^" +
				// заменяем wildcard: матчим любой символ, кроме '/'
				strings.Replace(pref, "*", "[^/]+", -1)

		if i != len(urlPrefixes)-1 {
			rawRegexp += "|"
		}
	}

	return rawRegexp
}
