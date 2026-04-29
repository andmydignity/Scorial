# Info

atom directory: Contains atom.tmpl and the generated atom.xml.br (brotli encoded). You can modify the atom.tmpl to change how the Atom feed is generated.

commonTemplates directory: All templates in this directory are used in both home page and generated posts.

homePage directory: All templates in this directory are used in home page. Generated home page (home.html.br) is also stored here. A base.tmpl file is required.

media directory: Medias (images, videos,audios etc..) are stored here. Files here are served by a file server accesible in /media

style directory:

templates: All templates in this directory are used in posts generated from .md files. A base.tmpl file is required. If mainContentInAtomFeed is true, a <main> tag is also required.

posts: Where generated posts from .md files are stored.
