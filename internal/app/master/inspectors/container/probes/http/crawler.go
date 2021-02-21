package http

import (
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
)

const (
	defaultCrawlMaxDepth         = 3
	defaultCrawlMaxPageCount     = 1000
	defaultCrawlConcurrency      = 10
	defaultMaxConcurrentCrawlers = 1
)

func (p *CustomProbe) crawl(proto, domain, addr string) {
	p.workers.Add(1)

	var httpClient *http.Client
	if strings.HasPrefix(proto, config.ProtoHTTP2) {
		httpClient = getHTTPClient(proto)
		httpClient.Timeout = 10 * time.Second //matches the timeout used by Colly
		jar, _ := cookiejar.New(nil)
		httpClient.Jar = jar
	}

	if p.maxConcurrentCrawlers > 0 &&
		p.concurrentCrawlers != nil {
		p.concurrentCrawlers <- struct{}{}
	}

	go func() {
		defer func() {
			if p.maxConcurrentCrawlers > 0 &&
				p.concurrentCrawlers != nil {
				<-p.concurrentCrawlers
			}

			p.workers.Done()
		}()

		var pageCount int

		c := colly.NewCollector()
		c.UserAgent = "ds.crawler"
		c.IgnoreRobotsTxt = true
		c.Async = true
		c.AllowedDomains = []string{domain}
		c.AllowURLRevisit = false
		if httpClient != nil {
			c.SetClient(httpClient)
		}

		if p.crawlMaxDepth > 0 {
			c.MaxDepth = p.crawlMaxDepth
		}

		if p.crawlConcurrency > 0 {
			c.Limit(&colly.LimitRule{
				DomainGlob:  "*",
				Parallelism: p.crawlConcurrency,
			})
		}

		c.OnHTML("a[href]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML(a[href]) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			e.Request.Visit(e.Attr("href"))
		})

		c.OnHTML("link[href]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML(link[href]) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			switch e.Attr("rel") {
			case "dns-prefetch", "preconnect", "alternate":
				return
			}

			e.Request.Visit(e.Attr("href"))
		})

		c.OnHTML("script[src], source[src], img[src]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML(script/source/img) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			e.Request.Visit(e.Attr("src"))
		})

		c.OnHTML("source[srcset]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML(source[srcset]) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			e.Request.Visit(e.Attr("srcset"))
		})

		c.OnHTML("[data-src]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML([data-src]) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			e.Request.Visit(e.Attr("data-src"))
		})

		c.OnRequest(func(r *colly.Request) {
			p.xc.Out.Info("http.probe.crawler",
				ovars{
					"page": pageCount,
					"url":  r.URL,
				})

			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				p.xc.Out.Info("http.probe.crawler.stop",
					ovars{
						"reason": "reached max visits",
					})

				log.Debugf("http.CustomProbe.crawl.OnRequest - reached max page count (%v)", p.crawlMaxPageCount)
				r.Abort()
				return
			}

			pageCount++
		})

		c.OnError(func(_ *colly.Response, err error) {
			log.Tracef("http.CustomProbe.crawl - error=%v", err)
		})

		c.Visit(addr)
		c.Wait()
		p.xc.Out.Info("probe.crawler.done",
			ovars{
				"addr": addr,
			})
	}()
}
