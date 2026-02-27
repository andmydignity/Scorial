/* common.js — no dependencies */
(function () {
  "use strict";

  /* ── Theme ─────────────────────────────────────────────────────────── */
  var THEME_KEY = "theme";

  function getPreferred() {
    var saved = localStorage.getItem(THEME_KEY);
    if (saved) return saved;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function applyTheme(theme) {
    if (theme === "dark") {
      document.documentElement.setAttribute("data-theme", "dark");
    } else {
      document.documentElement.removeAttribute("data-theme");
    }
  }

  // Apply immediately (before DOMContentLoaded) to avoid flash
  applyTheme(getPreferred());

  /* ── Sidebar state ──────────────────────────────────────────────────── */
  var SIDEBAR_KEY = "sidebar";

  function getSidebarState() {
    return localStorage.getItem(SIDEBAR_KEY) !== "hidden";
  }

  function applySidebar(visible) {
    if (visible) {
      document.body.classList.remove("sidebar-hidden");
    } else {
      document.body.classList.add("sidebar-hidden");
    }
  }

  /* ── Build TOC in sidebar from page headings ────────────────────────── */
  function buildSidebarTOC() {
    var contentNav = document.querySelector(".content > nav");
    var sidebar = document.querySelector(".sidebar");
    if (!sidebar) return;

    var tocList = contentNav ? parseTOCFromNav(contentNav) : buildTOCFromHeadings();
    if (!tocList || !tocList.children.length) return;

    var label = document.createElement("div");
    label.className = "sidebar-label";
    label.textContent = "On this page";

    sidebar.insertBefore(tocList, sidebar.firstChild);
    sidebar.insertBefore(label, sidebar.firstChild);
  }

  function parseTOCFromNav(nav) {
    // The parser emits a nested <ul> inside a <nav> — grab the root <ul>
    var srcUL = nav.querySelector("ul");
    if (!srcUL) return null;

    var ul = document.createElement("ul");
    ul.className = "sidebar-toc";

    function processItems(srcList, destList, depth) {
      var items = srcList.children;
      for (var i = 0; i < items.length; i++) {
        var item = items[i];
        var a = item.querySelector(":scope > a");
        if (!a) continue;

        var li = document.createElement("li");
        li.className = "toc-h" + (depth + 1);

        var newA = document.createElement("a");
        newA.href = a.getAttribute("href");
        newA.textContent = a.textContent;
        li.appendChild(newA);

        // recurse into nested <ul>
        var nested = item.querySelector(":scope > ul");
        if (nested) {
          var subUL = document.createElement("ul");
          subUL.className = "sidebar-toc";
          processItems(nested, subUL, depth + 1);
          li.appendChild(subUL);
        }

        destList.appendChild(li);
      }
    }

    processItems(srcUL, ul, 1);
    return ul;
  }

  function buildTOCFromHeadings() {
    var headings = document.querySelectorAll(".content h1, .content h2, .content h3, .content h4");
    if (!headings.length) return null;

    var ul = document.createElement("ul");
    ul.className = "sidebar-toc";

    headings.forEach(function (h) {
      var level = parseInt(h.tagName[1], 10);
      var li = document.createElement("li");
      li.className = "toc-h" + level;

      var a = document.createElement("a");
      a.href = "#" + h.id;
      a.textContent = h.textContent;
      li.appendChild(a);
      ul.appendChild(li);
    });

    return ul;
  }

  /* ── Scrollspy: highlight active TOC link ───────────────────────────── */
  function initScrollspy() {
    var headings = Array.from(
      document.querySelectorAll(".content h1[id], .content h2[id], .content h3[id], .content h4[id]")
    );
    if (!headings.length) return;

    var tocLinks = document.querySelectorAll(".sidebar-toc a");

    function onScroll() {
      var scrollY = window.scrollY + 80; // offset for sticky nav
      var current = headings[0];

      for (var i = 0; i < headings.length; i++) {
        if (headings[i].offsetTop <= scrollY) {
          current = headings[i];
        }
      }

      tocLinks.forEach(function (a) {
        a.classList.toggle("active", a.getAttribute("href") === "#" + current.id);
      });
    }

    window.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
  }

  /* ── KaTeX rendering ────────────────────────────────────────────────── */
  function renderMath() {
    if (typeof katex === "undefined") {
      // Inject KaTeX CSS and JS together, only when math is actually on the page
      var KATEX = "https://cdn.jsdelivr.net/npm/katex@0.16.10/dist/";

      var link = document.createElement("link");
      link.rel = "stylesheet";
      link.href = KATEX + "katex.min.css";
      document.head.appendChild(link);

      var s = document.createElement("script");
      s.src = KATEX + "katex.min.js";
      s.onload = doRender;
      document.head.appendChild(s);
    } else {
      doRender();
    }
  }

  function doRender() {
    // Inline math: <span class="math inline">\(...\)</span>
    document.querySelectorAll(".math.inline").forEach(function (el) {
      var src = el.textContent || el.innerText;
      // Strip surrounding \( \) or $ $ delimiters if present
      src = src.replace(/^\\\(/, "").replace(/\\\)$/, "")
        .replace(/^\$/, "").replace(/\$$/, "");
      try {
        katex.render(src, el, { throwOnError: false, displayMode: false });
      } catch (e) { /* leave as-is */ }
    });

    // Display math: <span class="math display">\[...\]</span>
    document.querySelectorAll(".math.display").forEach(function (el) {
      var src = el.textContent || el.innerText;
      src = src.replace(/^\\\[/, "").replace(/\\\]$/, "")
        .replace(/^\$\$/, "").replace(/\$\$$/, "");
      try {
        katex.render(src, el, { throwOnError: false, displayMode: true });
      } catch (e) { /* leave as-is */ }
    });
  }

  /* ── Active nav links ───────────────────────────────────────────────── */
  function markActive() {
    var path = window.location.pathname.replace(/\/$/, "") || "/";
    document.querySelectorAll(".navbar nav a[href]").forEach(function (a) {
      var href = a.getAttribute("href").replace(/\/$/, "") || "/";
      a.classList.toggle("active", href === path);
    });
  }

  /* ── Anchor highlight on hash navigation ────────────────────────────── */
  function highlightAnchor() {
    if (!window.location.hash) return;
    var el = document.querySelector(window.location.hash);
    if (!el) return;
    el.style.transition = "background 700ms ease";
    el.style.background = "var(--accent-dim)";
    el.style.borderRadius = "3px";
    setTimeout(function () { el.style.background = ""; }, 1800);
  }

  /* ── DOMContentLoaded ───────────────────────────────────────────────── */
  document.addEventListener("DOMContentLoaded", function () {

    /* --- Theme toggle button --- */
    var themeBtn = document.getElementById("theme-toggle");
    if (themeBtn) {
      themeBtn.addEventListener("click", function () {
        var current = document.documentElement.getAttribute("data-theme") === "dark"
          ? "dark" : "light";
        var next = current === "dark" ? "light" : "dark";
        applyTheme(next);
        localStorage.setItem(THEME_KEY, next);
      });
    }

    /* --- Sidebar toggle button --- */
    var sidebarBtn = document.getElementById("sidebar-toggle");
    var isMobile = window.innerWidth <= 768;

    if (sidebarBtn && !isMobile) {
      applySidebar(getSidebarState());

      sidebarBtn.addEventListener("click", function () {
        var isVisible = !document.body.classList.contains("sidebar-hidden");
        applySidebar(!isVisible);
        localStorage.setItem(SIDEBAR_KEY, isVisible ? "hidden" : "visible");
      });
    } else {
      // Mobile — remove sidebar-hidden so it doesn't interfere
      document.body.classList.remove("sidebar-hidden");
    }

    // Mobile nav hamburger
    var hamburger = document.getElementById("nav-hamburger");
    var mainNav = document.getElementById("main-nav");
    if (hamburger && mainNav) {
      hamburger.addEventListener("click", function () {
        var isOpen = mainNav.classList.toggle("open");
        hamburger.setAttribute("aria-expanded", isOpen ? "true" : "false");

        // On first open, inject TOC into the mobile menu if not already there
        if (isOpen && !mainNav.querySelector(".mobile-toc")) {
          var sidebar = document.querySelector(".sidebar");
          if (sidebar && sidebar.innerHTML.trim()) {
            var divider = document.createElement("div");
            divider.className = "mobile-toc-divider";

            var clone = sidebar.cloneNode(true);
            clone.className = "mobile-toc";

            mainNav.appendChild(divider);
            mainNav.appendChild(clone);
          }
        }
      });
      // Close menu when a link is tapped
      mainNav.addEventListener("click", function (e) {
        if (e.target.tagName === "A") {
          mainNav.classList.remove("open");
          hamburger.setAttribute("aria-expanded", "false");
        }
      });
    }

    // Remove the stray {:toc} paragraph the Markdown parser leaves behind
    document.querySelectorAll(".content p").forEach(function (p) {
      if (p.textContent.trim() === "{:toc}") p.remove();
    });

    buildSidebarTOC();
    initScrollspy();
    markActive();
    highlightAnchor();

    /* Render math if any .math.inline elements exist */
    if (document.querySelector(".math.inline, .math.display")) {
      renderMath();
    }
  });

})();
