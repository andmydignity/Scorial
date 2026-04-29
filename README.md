
![Logo](https://raw.githubusercontent.com/andmydignity/Scorial/refs/heads/main/Scorial.png)


# Scorial

A simple blazing fast Markdown based CMS designed for people whom just want to publish their work.

## Acknowledgements

 It's currently in beta, it's subject to major changes. 


## Features
- Just a single binary, no dependency management.
- Generate posts directly from markdown files.
- Regenerate posts automatically when modified. (FS watcher) 
- Automatic Atom feed generation.
- Automatic TLS certs with caddy.
- Option to pass the main content of the post to the Atom feed.
- Built-in rate limit middleware (off by default).
- Support for frontmatter features: date, category, draft, createdAt, modifedAt.




## Optimizations
- Built with Go.
- All pages are static.
- Posts and Atom feed have Etags, reducing the load.
- All pages and atom feed are compressed with Brotli on build time.
- A LRU (Least Recently Used) cache for posts for faster loads.
- Home page and Atom feed cached on RAM.


## Roadmap

- A basic Dashboard (maybe)
- Add core Obsidian features (like wikilinks)
- A sync script for syncing in between PC and the server.
- WebFinger
- Embedded local media handling
- Docker deployment.
- Actually good README.


## Installation

Just download the latest release, and run the "app". Don't forget to configure the conf.yaml file! You can also change the page templates to your liking.
    
