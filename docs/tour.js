// The tour's drawn lich window. Each tab plays a short story in the window's main area; opening a tab starts
// its story from the top, and the tabs move on by themselves once a story has had its time.
(function () {
  var tour = document.getElementById("tour"), wall = document.getElementById("wall");
  var radios = Array.prototype.slice.call(tour.querySelectorAll('input[name="tour"]'));
  var seg = tour.querySelector(".seg"), cards = wall.querySelectorAll(".scard");
  var labels = JSON.parse(wall.getAttribute("data-labels"));
  var still = matchMedia("(prefers-reduced-motion: reduce)").matches;
  var shown = false;

  var span = function (cls) { return function (t) { return '<span class="' + cls + '">' + t + "</span>"; }; };
  var ok = span("ok"), bad = span("bad"), dim = span("dim"), blue = span("blue"), amber = span("amber"), mut = span("mut"), pink = span("pink");
  var jitter = function (lo, hi) { return lo + Math.random() * (hi - lo); };

  function active() { return radios.findIndex(function (r) { return r.checked; }); }
  function setCard(k) { cards.forEach(function (c, i) { c.classList.toggle("on", i === k); }); }
  function setFooter(branch, provider, model, add, del) {
    document.getElementById("wall-branch").textContent = branch;
    document.getElementById("wall-model").textContent = model;
    // A clean checkout's chip is the diff glyph and "0"; a changed one counts files and lines.
    wall.querySelector(".fchip").innerHTML = add || del
      ? '<svg class="ic"><use href="#i-file-text"/></svg><span>1</span><i>·</i><span class="ds"><span class="add">+' + add +
        '</span><span class="del">-' + del + "</span></span>"
      : '<svg class="ic"><use href="#i-diff"/></svg><span>0</span>';
    wall.querySelector(".foot .model use").setAttribute("href", "#p-" + provider);
  }
  // A card's status ring: busy spins while the agent works, done until the card is looked at, read after.
  function setRing(k, state) { cards[k].querySelector(".ring-st").className = "ring-st " + state; }
  var rungIcon = { Bash: "terminal", Read: "file-text", Edit: "pencil", Write: "pencil" };
  function setRung(k, tool) {
    var parts = tool.split(" · ");
    cards[k].querySelector(".rung").innerHTML = '<svg class="ic"><use href="#i-' + rungIcon[parts[0]] + '"/></svg><b>' +
      parts[0] + '</b><i>·</i><span class="mono">' + parts[1] + "</span>";
  }

  // A story awaits wait(ms) between its beats. The wait holds while its tab is closed or the window is off screen,
  // and throws once the tab is opened again so the story starts over.
  var generation = [0, 0, 0, 0, 0, 0, 0];
  // wait.end() marks the story told once; the tab moves on only after that, whatever its bar says.
  var told = [false, false, false, false, false, false, false], barDone = false;
  function waiter(v) {
    var mine = generation[v];
    var wait = function (ms) {
      return new Promise(function (done, restart) {
        (function tick() {
          if (generation[v] !== mine) return restart();
          if (shown && !document.hidden && active() === v) {
            return setTimeout(function () {
              if (generation[v] !== mine) return restart();
              active() === v ? done() : tick();
            }, ms);
          }
          setTimeout(tick, 100);
        })();
      });
    };
    // end marks the story told and then holds it on its last frame until its tab opens again or another does,
    // so a tab whose bar outlasts its story never shows the story start over and get cut off.
    wait.end = function () {
      if (generation[v] === mine) {
        told[v] = true;
        advance();
      }
      return new Promise(function (done, restart) {
        (function hold() {
          if (generation[v] !== mine) return restart();
          setTimeout(hold, 200);
        })();
      });
    };
    return wait;
  }

  // Terminals, drawn the way each CLI draws its own: scrollback, then the input and the status line under it.
  var chrome = {
    claude: {
      asked: function (p) { return mut("❯ " + p); },
      bottom: function (input, t) {
        // Claude Code's input box: a rule naming the worktree, the prompt, a closing rule, then its mode, indented.
        return ['<span class="rule">' + (t.rule || "docs-mcp-matrix-68c5") + ' ─</span>', mut("❯") + " " + input, '<span class="rule"></span>',
          "  " + amber("⏵⏵ auto mode on") + " " + dim("(shift+tab to cycle)")];
      }
    },
    codex: {
      asked: function (p) { return mut("› " + p); },
      bottom: function (input) {
        return ["› " + (input || dim("Ask Codex to do anything")), "",
          amber("gpt-5-codex medium") + " · " + blue("Workspace") + " · Ask for approval"];
      }
    },
    // opencode: the message in a box with a pink left bar, a blank row above and below it; the input box holds
    // the prompt row and the agent's status row, with the path and the token count under it.
    opencode: {
      asked: function (p) { return '<span class="box">\n' + p + "\n</span>"; },
      bottom: function (input) {
        return ['<span class="box">\n' + input + "\n\n" + pink("Build") + " " + mut("auto") + " · claude-sonnet-4.5 " + dim("Anthropic") +
            " · " + '<b class="amber">high</b></span>',
          '<span class="oc-foot">' + mut("~/…/fix-relay-deadline") + "<span>" + mut("12.8K (6%)") + "  <b>ctrl+p</b> " + mut("commands") + "</span></span>"];
      }
    },
    crush: {
      asked: function (p) { return '<span class="cbar">' + p + "</span>"; },
      bottom: function (input, t) {
        var keys = [["tab", "focus chat"], ["shift+tab", "mode"], ["/ or ctrl+p", "commands"], ["ctrl+m", "models"]];
        return ['<span class="yb">!</span> ' + input + (t.input ? "" : dim("Go crazy")), '<span class="yl">:::</span>', '<span class="yl">:::</span>', "",
          '<span class="clip"><span class="yb">YOLO MODE</span>  ' + keys.map(function (k) { return k[0] + " " + dim(k[1]); }).join(dim(" • ")) + "</span>"];
      }
    }
  };
  function terminal(pre, provider, lines) {
    var t = { pre: pre, provider: provider, lines: lines.slice(), input: "" };
    t.draw = function (focused) {
      var input = t.input + (focused ? '<span class="cur"></span>' : "");
      t.pre.innerHTML = t.lines.slice(-30).concat("", chrome[provider].bottom(input, t)).map(function (l) {
        return '<span class="l">' + (l || " ") + "</span>";
      }).join("");
    };
    t.type = async function (wait, text, lo, hi) {
      for (var c of text) { t.input += c; t.draw(true); await wait(jitter(lo, hi)); }
    };
    t.submit = function (prompt) { t.input = ""; t.lines.push("", chrome[provider].asked(prompt)); t.draw(true); };
    t.stream = async function (wait, out) {
      for (var line of out) { await wait(jitter(220, 600)); t.lines.push(line); t.draw(true); }
    };
    return t;
  }

  // 1. Mix agents: four sessions on one wall, taking turns.
  var wallSessions = [
    { provider: "claude", model: "opus 5.5 · medium", runs: [
      { prompt: "add an MCP row to every README table", tool: "Edit · README.md", add: 12, del: 1, out: [
        "● Update(README.md)", "  ⎿  " + ok("+ | MCP server | yes | yes | yes | …"),
        "● Update(README.pt-BR.md)", "  ⎿  " + ok("+ | Servidor MCP | sim | sim | …"),
        "● All 7 READMEs carry the same row.", dim("✻ Crunched for 48s · done 20:24")] },
      { prompt: "does the intro still call Cursor an MCP host?", tool: "Read · README.md", add: 14, del: 2, out: [
        "● Read(README.md)", "  ⎿  Read 212 lines",
        "● Line 36 says \"Cursor CLI (MCP)\", but lich hands", "  Cursor no MCP server. Fixed the label:",
        "  ⎿  " + bad("- Cursor CLI (MCP)") + "  " + ok("+ Cursor CLI"), dim("✻ Crunched for 21s · done 20:26")] }] },
    { provider: "codex", model: "gpt-5-codex · medium", runs: [
      { prompt: "a token that expires right now is still accepted", tool: "Bash · go test ./internal/auth", add: 2, del: 2, out: [
        "• Explored", "  └ Read token.go", "• Edited internal/auth/token.go (" + ok("+1") + " " + bad("-1") + ")",
        "    " + bad("- if exp.Before(now) {"), "    " + ok("+ if !exp.After(now) {"),
        "• Ran go test ./internal/auth", "  └ ok  internal/auth  0.41s", dim("Worked for 16s • 8:31 PM")] },
      { prompt: "add a test for the exact expiry instant", tool: "Edit · token_test.go", add: 14, del: 2, out: [
        "• Edited internal/auth/token_test.go (" + ok("+12") + ")", "    " + ok("+ func TestExpiresAtNow(t *testing.T) {"),
        "• Ran go test ./internal/auth -run Expires", "  └ ok  internal/auth  0.38s", dim("Worked for 22s • 8:33 PM")] }] },
    { provider: "opencode", model: "claude-sonnet-4.5", runs: [
      { prompt: "the relay deadline outlives its context", tool: "Edit · ticket.go", add: 9, del: 3, out: [
        "", "  " + span("thought")("+ Thought: 1.8s"), "", "  → Read internal/relay/ticket.go",
        "  ← Edit internal/relay/ticket.go", "    " + ok("+ ctx, cancel := context.WithTimeout(ctx, d)"),
        "    " + ok("+ defer cancel()"), "", "  The deadline comes from the caller now;", "  all 31 tests pass.", "",
        "  " + pink("▣") + " Build " + dim("· claude-sonnet-4.5 · 17.6s")] },
      { prompt: "why did the old one never fire?", tool: "Read · relay.go", add: 9, del: 3, out: [
        "", "  " + span("thought")("+ Thought: 0.9s"), "", "  → Read internal/relay/relay.go", "",
        "  The timer was armed with context.Background(),", "  so cancelling the request never reached it.", "",
        "  " + pink("▣") + " Build " + dim("· claude-sonnet-4.5 · 6.2s")] }] },
    { provider: "crush", model: "glm-4.6", runs: [
      { prompt: "document what a broken theme does", tool: "Write · docs/themes.md", add: 24, del: 0, out: [
        "", "Thought for " + dim("2s"), "", ok("✓") + " View " + dim("docs/themes.md"), ok("✓") + " Edit " + dim("docs/themes.md"),
        "  " + ok("+ A theme that fails the schema stays listed in Settings › Appearance, marked unloadable."), "",
        "Documented the rule and the error it shows.", "", '<span class="cmeta"><span>' + dim("◇") + "GLM 4.6 " + dim("via Z.ai in 12s") + "</span></span>"] },
      { prompt: "link the schema from the README", tool: "Edit · README.md", add: 26, del: 0, out: [
        "", "Thought for " + dim("1s"), "", ok("✓") + " Edit " + dim("README.md"), "  " + ok("+ [theme schema](themes/lich-theme.schema.json)"), "",
        "Linked it under Themes.", "", '<span class="cmeta"><span>' + dim("◇") + "GLM 4.6 " + dim("via Z.ai in 6s") + "</span></span>"] }] }
  ];
  var wallTiles = wall.querySelectorAll(".v2 .tile");
  var wallTerms = wallSessions.map(function (s, k) {
    var first = s.runs[0];
    return terminal(wallTiles[k].querySelector("pre"), s.provider, [chrome[s.provider].asked(first.prompt)].concat(first.out));
  });
  function focusWall(k, run) {
    wallTiles.forEach(function (t, i) { t.classList.toggle("on", i === k); });
    setCard(k);
    setFooter(cards[k].querySelector(".t b").textContent, wallSessions[k].provider, wallSessions[k].model, run.add, run.del);
    wallTerms.forEach(function (t, i) { t.draw(i === k); });
  }
  var wallNext = [1, 1, 1, 1];
  async function mixAgents(wait) {
    for (var k = 0; ; k = (k + 1) % wallSessions.length) {
      var s = wallSessions[k], t = wallTerms[k], run = s.runs[wallNext[k]];
      focusWall(k, s.runs[(wallNext[k] + 1) % s.runs.length]);
      wallNext[k] = (wallNext[k] + 1) % s.runs.length;
      await wait(700);
      await t.type(wait, run.prompt, 30, 75);
      await wait(350);
      t.submit(run.prompt);
      setRung(k, run.tool);
      setRing(k, "busy");
      setFooter(cards[k].querySelector(".t b").textContent, s.provider, s.model, run.add, run.del);
      await t.stream(wait, run.out);
      setRing(k, "done");
      cards[k].querySelector(".rung").innerHTML = "";
      await wait(1600);
      setRing(k, "read");
      if (k === 1) await wait.end();
    }
  }

  // 2. Review the diff: the footer's changes chip slides the dock in beside the session's terminal and full screen
  // gives it the whole panel. There, select lines, comment for the session from the context menu and add it to the
  // batch; Send pastes the batch into the terminal, where the prompt waits for you to send it.
  var rv = wall.querySelector(".v3"), dock = rv.querySelector(".dock"), diffRows = dock.querySelectorAll(".diff .r");
  var chip = wall.querySelector(".fchip"), full = dock.querySelector(".full"), close = dock.querySelector(".close");
  var reviewTerm = terminal(rv.querySelector(".rv-term pre"), "claude", wallSessions[0].runs[0].out);
  var batch = dock.querySelector(".batch"), cmts = dock.querySelector(".cmts"), count = dock.querySelector(".count");
  var send = dock.querySelector(".send"), menu = dock.querySelector(".menu");
  var comments = [
    { rows: [2, 3, 4, 5], lines: "9-10", changed: 4, text: "Say how many: \"six more\" ages better than a list of names." },
    { rows: [10], lines: "23", changed: 1, text: "Link the MCP row to docs/cli.md#mcp." }
  ];
  function setFull(on) {
    rv.classList.toggle("full", on);
    full.querySelector("use").setAttribute("href", on ? "#i-minimize-2" : "#i-maximize-2");
  }
  function resetReview() {
    diffRows.forEach(function (r) { r.classList.remove("sel"); });
    dock.querySelectorAll(".composer").forEach(function (e) { e.remove(); });
    cmts.innerHTML = ""; batch.classList.remove("has"); send.classList.remove("press");
    menu.classList.remove("show");
  }
  // tap draws a ring where the tour clicks, so a viewer sees what was pressed; every simulated click goes through it.
  function tap(el) {
    var w = wall.getBoundingClientRect(), r = el.getBoundingClientRect(), ring = document.createElement("span");
    ring.className = "tap";
    ring.style.left = r.left - w.left + r.width / 2 + "px";
    ring.style.top = r.top - w.top + r.height / 2 + "px";
    wall.appendChild(ring);
    setTimeout(function () { ring.remove(); }, 400);
  }
  async function press(wait, el) { tap(el); el.classList.add("press"); await wait(220); el.classList.remove("press"); }
  async function reviewDiff(wait) {
    for (;;) {
      resetReview();
      rv.classList.remove("open"); setFull(false); chip.classList.remove("on");
      reviewTerm.lines = [chrome.claude.asked(wallSessions[0].runs[0].prompt)].concat(wallSessions[0].runs[0].out);
      reviewTerm.draw(true);
      setCard(0); setFooter("docs/mcp-matrix", "claude", "opus 5.5 · medium", 7, 3);
      await wait(1200);
      tap(chip); chip.classList.add("on"); rv.classList.add("open");
      await wait(1100);
      await press(wait, full); setFull(true);
      for (var n = 0; n < comments.length; n++) {
        var c = comments[n], last = diffRows[c.rows[c.rows.length - 1]];
        await wait(700);
        for (var i of c.rows) { diffRows[i].classList.add("sel"); await wait(45); }
        await wait(250);
        menu.querySelector(".m-lines").textContent = "Inject lines " + c.lines;
        menu.querySelector(".m-cmt").textContent = "Comment for the session " + c.lines + "…";
        menu.querySelector(".m-rev").textContent = "Revert " + c.changed + " changed line" + (c.changed > 1 ? "s" : "");
        menu.style.top = last.offsetTop + last.offsetHeight + "px";
        menu.classList.add("show");
        await wait(450);
        menu.querySelector(".m-cmt").classList.add("hover");
        await wait(300);
        tap(menu.querySelector(".m-cmt"));
        menu.querySelector(".m-cmt").classList.remove("hover");
        menu.classList.remove("show");
        var box = document.createElement("div");
        box.className = "composer";
        box.innerHTML = '<div class="hd">for the session · ' + c.lines + '</div>' +
          '<div class="ta"><span class="tx"></span><span class="ph">What should change here? It goes to the session with the rest.</span></div>' +
          '<div class="bt"><span class="add-b">Add to batch</span><span class="btn-g">Cancel</span></div>';
        last.after(box);
        var tx = box.querySelector(".tx");
        await wait(250);
        box.classList.add("typing");
        for (var ch of c.text) { tx.textContent += ch; await wait(jitter(22, 55)); }
        await wait(450);
        await press(wait, box.querySelector(".add-b"));
        box.remove();
        c.rows.forEach(function (i) { diffRows[i].classList.remove("sel"); });
        cmts.insertAdjacentHTML("beforeend", '<div class="c"><span class="at">README.md:' + c.lines + "</span><span>" + c.text +
          '</span><svg class="ic"><use href="#i-pencil"/></svg><svg class="ic"><use href="#i-x"/></svg></div>');
        batch.classList.add("has");
        count.textContent = (n + 1) + (n ? " comments" : " comment");
      }
      await wait(1200);
      // Send submits the batch as the session's prompt; the strip stays until the session has taken it.
      await press(wait, send);
      reviewTerm.submit("[Pasted text #1 +4 lines]");
      await wait(700);
      resetReview();
      await wait(700);
      await press(wait, full); setFull(false);
      await wait(900);
      await press(wait, close); rv.classList.remove("open"); chip.classList.remove("on");
      await wait(500);
      await reviewTerm.stream(wait, ["● Update(README.md)", "  ⎿  " + ok("+ Run Claude Code, Codex and six more agent CLIs"),
        "  ⎿  " + ok("+ | [MCP server](docs/cli.md#mcp) | yes | yes | yes |"), "● Both review comments are in.", dim("✻ Crunched for 26s · done 20:41")]);
      await wait(2200);
      await wait.end();
    }
  }

  // 3. Pull requests: open one, watch its checks pass on the Checks tab, then merge with the method you pick.
  var pulls = wall.querySelector(".v4"), prRows = pulls.querySelectorAll(".pr"), checks = pulls.querySelectorAll(".ck");
  var merge = pulls.querySelector(".merge"), state = pulls.querySelector(".state"), cks = pulls.querySelector(".cks");
  var mmenu = pulls.querySelector(".mmenu"), tabOv = pulls.querySelector(".t-ov"), tabCk = pulls.querySelector(".t-ck");
  var bodyOv = pulls.querySelector(".pd-b.ov"), bodyCk = pulls.querySelector(".pd-b.ckl");
  var icon = function (name) { return '<svg class="ic"><use href="#i-' + name + '"/></svg>'; };
  function showTab(checksTab) {
    tabOv.classList.toggle("on", !checksTab); tabCk.classList.toggle("on", checksTab);
    bodyOv.classList.toggle("show", !checksTab); bodyCk.classList.toggle("show", checksTab);
  }
  // The list row's checks chip follows the open pull request's checks.
  function rowChip(k, done) {
    var c = prRows[k].querySelector(".pchk"), total = checks.length;
    c.className = done < total ? "pchk run" : "pchk ok";
    c.innerHTML = (done < total ? '<span class="sring"></span>' : icon("check")) + done + "/" + total;
  }
  function running(done) {
    cks.className = "cks running";
    cks.innerHTML = '<span class="sring"></span>' + (checks.length - done) + " of " + checks.length + " checks running";
  }
  function openPr(k) {
    var row = prRows[k];
    prRows.forEach(function (r, i) { r.classList.toggle("on", i === k); });
    pulls.querySelector(".num").textContent = row.querySelector(".num-l").textContent;
    pulls.querySelector(".ttl").textContent = row.querySelector(".tl").textContent;
    pulls.querySelector(".br").textContent = row.getAttribute("data-br");
    pulls.querySelector(".desc").textContent = row.getAttribute("data-desc");
    checks.forEach(function (c) { c.classList.remove("passed"); c.querySelector(".dur").textContent = ""; });
    state.className = "state"; state.innerHTML = icon("git-pull-request-arrow") + "Open";
    merge.classList.add("off"); mmenu.classList.remove("show");
    running(0); rowChip(k, 0); showTab(false);
  }
  async function pullRequests(wait) {
    for (var k = 0; ; k = (k + 1) % prRows.length) {
      if (prRows[k].querySelector(".pchk.bad")) continue;
      setCard(-1);
      openPr(k);
      await wait(1600);
      tap(tabCk); showTab(true);
      for (var i = 0; i < checks.length; i++) {
        await wait(jitter(350, 700));
        checks[i].classList.add("passed");
        checks[i].querySelector(".dur").textContent = Math.round(jitter(40, 240)) + "s";
        running(i + 1);
        rowChip(k, i + 1);
      }
      cks.className = "cks passed";
      cks.innerHTML = icon("check") + checks.length + " checks passed";
      merge.classList.remove("off");
      await wait(900);
      await press(wait, merge);
      mmenu.classList.add("show");
      await wait(600);
      mmenu.querySelector(".m-sq").classList.add("hover");
      await wait(450);
      tap(mmenu.querySelector(".m-sq"));
      mmenu.querySelector(".m-sq").classList.remove("hover");
      mmenu.classList.remove("show");
      state.className = "state merged"; state.innerHTML = icon("git-merge") + "Merged";
      await wait(2200);
      await wait.end();
    }
  }

  // 4. Hand off work: a Claude Code session gives a Codex session a task over lich's MCP tools and reads the answer.
  var hand = wall.querySelectorAll(".v5 .tile");
  async function handOff(wait) {
    for (;;) {
      var asker = terminal(hand[0].querySelector("pre"), "claude", [dim("✻ Crunched for 48s · done 20:24")]);
      var worker = terminal(hand[1].querySelector("pre"), "codex", [dim("Worked for 22s • 8:33 PM")]);
      var focus = function (k) {
        hand.forEach(function (t, i) { t.classList.toggle("on", i === k); });
        setCard(k);
        if (k) setFooter("fix/auth-expiry", "codex", "gpt-5-codex · medium", 14, 2);
        else setFooter("docs/mcp-matrix", "claude", "opus 5.5 · medium", 12, 1);
        asker.draw(k === 0); worker.draw(k === 1);
      };
      var ask = "have fix/auth-expiry test the exact expiry instant, then document the rule";
      focus(0);
      await wait(800);
      await asker.type(wait, ask, 25, 60);
      await wait(350);
      asker.submit(ask);
      await asker.stream(wait, ["● lich - send_to_session (MCP)(session: \"fix/auth-expiry\")", "  ⎿  Sent · ticket a41c",
        "● lich - wait_for_answer (MCP)(ticket: \"a41c\")", "  ⎿  " + amber("Waiting on fix/auth-expiry…")]);
      await wait(700);
      focus(1);
      var task = "add a test for the exact expiry instant and reply with what changed";
      await wait(400);
      await worker.type(wait, task, 8, 16);
      await wait(250);
      worker.submit(task);
      await worker.stream(wait, ["• Edited internal/auth/token_test.go (" + ok("+12") + ")",
        "    " + ok("+ func TestExpiresAtNow(t *testing.T) {"), "• Ran go test ./internal/auth", "  └ ok  internal/auth  0.38s",
        "• Replied to docs/mcp-matrix", dim("Worked for 19s • 8:35 PM")]);
      await wait(700);
      focus(0);
      asker.lines.pop();
      asker.lines.push("  ⎿  fix/auth-expiry: added TestExpiresAtNow; ok");
      asker.draw(true);
      await asker.stream(wait, ["● Update(docs/auth.md)", "  ⎿  " + ok("+ A token is expired at its exp instant."),
        dim("✻ Crunched for 1m 2s · done 20:35")]);
      await wait(2600);
      await wait.end();
    }
  }

  // 0. Start a task, as lich does it: New Session's menu, Worktree, then the dialog. "#766" looks the issue up and
  // names the branch after its title; Create makes the worktree, the dialog closes, the session's card appears at the
  // bottom of the sidebar under its own group, and once the agent is ready lich sends it the issue as its prompt.
  var solo = terminal(wall.querySelector(".v1 pre"), "claude", wallSessions[0].runs[0].out);
  var ns = wall.querySelector(".ns"), nmenu = wall.querySelector(".nmenu"), dlg = wall.querySelector(".wdlg");
  var nameIn = dlg.querySelector(".inp"), nameTx = nameIn.querySelector(".tx"), iss = dlg.querySelector(".iss");
  var hint = dlg.querySelector(".hint"), fresh = dlg.querySelector(".fresh"), create = dlg.querySelector(".crt");
  var list = wall.querySelector(".side .list");
  var issue = { number: 766, title: "Kiro CLI transcripts ignore KIRO_HOME", branch: "766-kiro-cli-transcripts-ignore" };
  // The new card's age counts up the way lich spells it ("{n}s", then "{n}m").
  var ageTimer = 0;
  function tickAge(el) {
    var born = Date.now();
    clearInterval(ageTimer);
    ageTimer = setInterval(function () {
      var secs = Math.floor((Date.now() - born) / 1000);
      el.textContent = secs < 60 ? secs + "s" : Math.floor(secs / 60) + "m";
    }, 1000);
  }
  function resetTask() {
    clearInterval(ageTimer);
    var added = list.querySelector(".new-task");
    if (added) added.remove();
    list.scrollTop = 0;
    ns.classList.remove("open"); nmenu.classList.remove("show"); nmenu.querySelector(".m-wt").classList.remove("hover");
    dlg.classList.remove("show"); nameIn.classList.remove("typing", "focus"); nameTx.textContent = "";
    iss.classList.remove("show"); hint.textContent = "branch: a random name"; fresh.textContent = "Checking the remote…";
    create.textContent = "Create"; create.classList.remove("off");
    solo.lines = [chrome.claude.asked(wallSessions[0].runs[0].prompt)].concat(wallSessions[0].runs[0].out);
    solo.input = ""; solo.rule = "";
    solo.draw(true);
  }
  function addTaskCard() {
    list.insertAdjacentHTML("beforeend", '<div class="new-task"><div class="grp"><span class="gt"><svg class="ic"><use href="#i-chevron-right"/></svg><span>' +
      issue.branch.toUpperCase() + '</span></span><i></i><span class="ib"><svg class="ic"><use href="#i-plus"/></svg></span><span class="ib"><svg class="ic"><use href="#i-ellipsis"/></svg></span></div>' +
      '<div class="scard on"><div class="t"><span class="ring-st none"><svg class="glyph"><use href="#p-claude"/></svg></span><b>' + issue.branch +
      '</b><small>0s</small></div><div class="m rung"></div><div class="m path"><span>&lrm;~/.local/share/lich/worktrees/e48f04f46a4a/' + issue.branch +
      '&lrm;</span></div><div class="m"><svg class="ic"><use href="#i-git-branch"/></svg><span class="brn">' + issue.branch + "</span></div></div></div>");
    list.scrollTop = list.scrollHeight;
    return list.querySelector(".new-task .scard");
  }
  async function startTask(wait) {
    for (;;) {
      resetTask();
      setCard(0); setFooter("docs/mcp-matrix", "claude", "opus 5.5 · medium", 12, 1);
      await wait(1000);
      tap(ns); ns.classList.add("open"); nmenu.classList.add("show");
      await wait(700);
      nmenu.querySelector(".m-wt").classList.add("hover");
      await wait(350);
      tap(nmenu.querySelector(".m-wt"));
      ns.classList.remove("open"); nmenu.classList.remove("show");
      dlg.classList.add("show"); nameIn.classList.add("focus");
      await wait(700);
      nameIn.classList.add("typing");
      for (var ch of "#" + issue.number) { nameTx.textContent += ch; hint.textContent = "branch: " + nameTx.textContent.slice(1); await wait(jitter(90, 150)); }
      await wait(400);
      hint.textContent = "Looking up #" + issue.number + "…";
      fresh.textContent = "2 behind origin/main · starts from origin/main";
      await wait(800);
      iss.textContent = "#" + issue.number + " " + issue.title; iss.classList.add("show");
      hint.textContent = "branch: " + issue.branch;
      await wait(1300);
      await press(wait, create);
      create.textContent = "Creating…"; create.classList.add("off");
      await wait(1300);
      dlg.classList.remove("show");
      cards.forEach(function (c) { c.classList.remove("on"); });
      var card = addTaskCard();
      tickAge(card.querySelector("small"));
      solo.lines = []; solo.input = ""; solo.rule = issue.branch + "-3f1a"; solo.draw(true);
      setFooter(issue.branch, "claude", "opus 5.5 · medium", 0, 0);
      await wait(1400);
      solo.input = dim("[Pasted text #1 +11 lines]");
      solo.draw(true);
      await wait(500);
      solo.submit("[Pasted text #1 +11 lines]");
      card.querySelector(".ring-st").className = "ring-st busy";
      var rung = function (tool, detail, icon) {
        card.querySelector(".rung").innerHTML = '<svg class="ic"><use href="#i-' + icon + '"/></svg><b>' + tool + '</b><i>·</i><span class="mono">' + detail + "</span>";
        list.scrollTop = list.scrollHeight;
      };
      rung("Read", "internal/terminal/transcript.go", "file-text");
      await solo.stream(wait, ["● Read(internal/terminal/transcript.go)", "  ⎿  Read 188 lines",
        "● kiroSessionPath builds the path from os.UserHomeDir()", "  alone, and KIRO_HOME moves the whole ~/.kiro tree."]);
      rung("Edit", "transcript.go", "pencil");
      await solo.stream(wait, ["● Update(internal/terminal/transcript.go)",
        "  ⎿  " + ok("+ if home := os.Getenv(\"KIRO_HOME\"); filepath.IsAbs(home) {"), "     " + ok("+   root = home")]);
      rung("Bash", "go test ./internal/terminal", "terminal");
      await solo.stream(wait, ["● Bash(go test ./internal/terminal)", "  ⎿  ok  internal/terminal  1.92s", dim("✻ Crunched for 1m 4s · done 20:52")]);
      card.querySelector(".ring-st").className = "ring-st done";
      setFooter(issue.branch, "claude", "opus 5.5 · medium", 9, 1);
      await wait(2600);
      await wait.end();
    }
  }

    // 5. Drive a browser: the agent's browser_open raises a toast whose Open gives the session a tab, its terminal
  // beside the browser pane; lich's arrow then glides to each action the agent sends, the toolbar's included.
  // The story's session is a signup-form fix, so the first card wears that name while it plays.
  var brw = wall.querySelector(".v6"), bpane = brw.querySelector(".bpane"), page = brw.querySelector(".pg");
  var acur = brw.querySelector(".acur"), aring = brw.querySelector(".aring"), url = brw.querySelector(".b-url");
  var btoast = wall.querySelector(".btoast"), tabs = wall.querySelector(".tabs");
  var email = page.querySelector(".pg-email"), pass = page.querySelector(".pg-pass"), err = page.querySelector(".pg-err");
  var signup = page.querySelector(".pg-btn");
  var browserTerm = terminal(brw.querySelector(".tile pre"), "claude", [dim("✻ Crunched for 12s · done 20:58")]);
  var BRANCH = "fix/signup-email";
  var first = { title: cards[0].querySelector(".t b").textContent, path: cards[0].querySelector(".path span").textContent,
    ds: cards[0].querySelector(".ds").innerHTML };
  // nameCard dresses the first card as the given session: its title, path, branch and diffstat.
  function nameCard(title, path, ds) {
    cards[0].querySelector(".t b").textContent = title;
    cards[0].querySelector(".brn").textContent = title;
    cards[0].querySelector(".path span").textContent = path;
    cards[0].querySelector(".ds").innerHTML = ds;
  }
  var signupPath = "\u200e~/.local/share/lich/worktrees/e48f04f46a4a/fix-signup-email\u200e";
  function resetBrowser() {
    brw.classList.remove("split"); btoast.classList.remove("show"); acur.classList.remove("on");
    page.className = "pg"; url.textContent = "localhost:5173/signup";
    resetNotes();
    [email, pass].forEach(function (i) { i.classList.remove("focus", "bad"); i.querySelector(".pg-tx").textContent = ""; });
    err.classList.remove("show"); signup.classList.remove("hit");
    var tab = tabs.querySelector(".btab"), web = cards[0].querySelector(".web");
    if (tab) tab.remove();
    if (web) web.remove();
    tabs.querySelector(".tab").classList.add("on");
    nameCard(first.title, first.path, first.ds);
    browserTerm.lines = [dim("✻ Crunched for 12s · done 20:58")];
    browserTerm.input = ""; browserTerm.rule = "fix-signup-email-2b7e";
    browserTerm.draw(true);
  }
  // glide moves the arrow to a point inside el (its center by default), in the pane's coordinates.
  function glide(el, fx, fy) {
    var p = bpane.getBoundingClientRect(), r = el.getBoundingClientRect();
    var x = r.left - p.left + r.width * (fx === undefined ? .5 : fx), y = r.top - p.top + r.height * (fy === undefined ? .5 : fy);
    acur.style.transform = "translate(" + x + "px," + y + "px)";
    return [x, y];
  }
  async function land(wait, at) {
    acur.classList.add("dip");
    aring.style.left = at[0] + "px"; aring.style.top = at[1] + "px";
    aring.classList.remove("go"); void aring.offsetWidth; aring.classList.add("go");
    await wait(90);
    acur.classList.remove("dip");
  }
  async function flash(wait, btn) { btn.classList.add("flash"); await wait(220); btn.classList.remove("flash"); }
  // fill clicks into a field and types text into it, as browser_type does.
  async function fill(wait, field, text) {
    var at = glide(field, .15);
    await wait(320);
    await land(wait, at);
    [email, pass].forEach(function (i) { i.classList.toggle("focus", i === field); });
    for (var ch of text) { field.querySelector(".pg-tx").textContent += ch; await wait(jitter(35, 70)); }
  }
  async function clickSignup(wait) {
    [email, pass].forEach(function (i) { i.classList.remove("focus"); });
    var at = glide(signup);
    await wait(380);
    await land(wait, at);
    signup.classList.add("hit");
    await wait(250);
    signup.classList.remove("hit");
  }
  function openBrowserTab() {
    tabs.querySelectorAll(".tab").forEach(function (t) { t.classList.remove("on"); });
    tabs.querySelector(".tab").insertAdjacentHTML("afterend", '<span class="tab on btab"><span class="tg"><svg class="ic"><use href="#i-columns-2"/></svg>' + BRANCH + "</span>" +
      '<span class="tx"><svg class="ic"><use href="#i-x"/></svg></span></span>');
    cards[0].querySelector(".ring-st").insertAdjacentHTML("afterend", '<svg class="ic web"><use href="#i-globe"/></svg>');
    brw.classList.add("split");
  }
  async function driveBrowser(wait) {
    for (;;) {
      resetBrowser();
      setCard(0); nameCard(BRANCH, signupPath, ""); setFooter(BRANCH, "claude", "opus 5.5 · medium", 0, 0);
      await wait(900);
      var ask = 'the signup form says "invalid email" for a valid one';
      await browserTerm.type(wait, ask, 25, 55);
      await wait(350);
      browserTerm.submit(ask);
      setRing(0, "busy");
      await browserTerm.stream(wait, ['● lich - browser_open (MCP)(url: "http://localhost:5173/signup")', "  ⎿  Opened · Create your account"]);
      btoast.classList.add("show");
      await wait(1500);
      await press(wait, btoast.querySelector(".bt-open"));
      btoast.classList.remove("show");
      openBrowserTab();
      await wait(600);
      glide(page, .5, .2); acur.classList.add("on");
      await wait(500);
      browserTerm.lines.push('● lich - browser_type (MCP)(field: "Email", text: "ana@shop.dev")'); browserTerm.draw(true);
      await fill(wait, email, "ana@shop.dev");
      browserTerm.lines.push('● lich - browser_type (MCP)(field: "Password", text: "••••••••")'); browserTerm.draw(true);
      await fill(wait, pass, "••••••••");
      browserTerm.lines.push('● lich - browser_click (MCP)(text: "Sign up")'); browserTerm.draw(true);
      await clickSignup(wait);
      email.classList.add("bad"); err.classList.add("show");
      await browserTerm.stream(wait, ['  ⎿  "Invalid email" under Email', "● Read(src/validate.ts)", "● Update(src/validate.ts)",
        "  ⎿  " + bad("- /^[a-z]+@[a-z]+\\.com$/"), "  ⎿  " + ok("+ /^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$/")]);
      setFooter(BRANCH, "claude", "opus 5.5 · medium", 1, 1);
      nameCard(BRANCH, signupPath, '<span class="add">+1</span><span class="del">-1</span>');
      browserTerm.lines.push("● lich - browser_reload (MCP)"); browserTerm.draw(true);
      var reload = brw.querySelector(".b-rel");
      await land(wait, glide(reload));
      await flash(wait, reload);
      [email, pass].forEach(function (i) { i.classList.remove("focus", "bad"); i.querySelector(".pg-tx").textContent = ""; });
      err.classList.remove("show");
      await wait(500);
      browserTerm.lines.push("● lich - browser_type (MCP)(Email, Password)"); browserTerm.draw(true);
      await fill(wait, email, "ana@shop.dev");
      await fill(wait, pass, "••••••••");
      browserTerm.lines.push('● lich - browser_click (MCP)(text: "Sign up")'); browserTerm.draw(true);
      await clickSignup(wait);
      page.classList.add("done");
      await browserTerm.stream(wait, ["  ⎿  Welcome, ana", "● The email check only took .com addresses; it now takes", "  any domain. Signed up as ana@shop.dev to confirm.",
        dim("✻ Crunched for 41s · done 21:03")]);
      setRing(0, "done");
      await wait(2600);
      await wait.end();
    }
  }

  var bpage = brw.querySelector(".bp-page"), pkRing = bpage.querySelector(".pk-ring"), pkTag = bpage.querySelector(".pk-tag");
  var pkPop = bpage.querySelector(".pk-pop"), pkTa = pkPop.querySelector(".pk-ta"), pkTray = bpage.querySelector(".pk-tray");
  var pkPins = bpage.querySelectorAll(".pk-pin"), pick = brw.querySelector(".b-pick");
  var heading = page.querySelector(".pg-form h1"), lead = page.querySelector(".pg-form p");
  var headingText = heading.textContent, leadText = lead.textContent;
  function resetNotes() {
    bpane.classList.remove("picking");
    [pkRing, pkTag, pkPop, pkTray].concat(Array.from(pkPins)).forEach(function (e) { e.classList.remove("on"); });
    bpage.querySelectorAll(".pk-ring.kept").forEach(function (e) { e.remove(); });
    pkTray.querySelector(".pk-rows").innerHTML = "";
    heading.textContent = headingText; lead.textContent = leadText; lead.hidden = false;
  }
  // box is el's rectangle in the page's coordinates, where the picker's overlays are placed.
  function box(el) {
    var p = bpage.getBoundingClientRect(), r = el.getBoundingClientRect();
    return { x: r.left - p.left, y: r.top - p.top, w: r.width, h: r.height };
  }
  // ring places a dashed outline 5px outside el, as noteRing does, and returns its box.
  function ring(r, el) {
    var b = box(el);
    b = { x: b.x - 5, y: b.y - 5, w: b.w + 10, h: b.h + 10 };
    Object.assign(r.style, { left: b.x + "px", top: b.y + "px", width: b.w + "px", height: b.h + "px" });
    return b;
  }
  // hover outlines el with marching dashes and names it in the tag above it, as the crosshair does under the pointer.
  function hover(el, label) {
    var b = box(el);
    ring(pkRing, el);
    pkRing.classList.add("march");
    pkTag.querySelector(".pk-l").textContent = label;
    pkTag.querySelector(".pk-c").textContent = "SignupForm";
    pkTag.querySelector(".pk-s").textContent = Math.round(b.w) + "×" + Math.round(b.h);
    pkTag.style.left = b.x + 3 + "px"; pkTag.style.top = b.y - 37 + "px";
    pkRing.classList.add("on"); pkTag.classList.add("on");
  }
  async function leaveNote(wait, el, label, n, note) {
    hover(el, label);
    await wait(1100);
    tap(el);
    pkTag.classList.remove("on"); pkRing.classList.remove("march");
    var b = box(el);
    pkPop.querySelector(".pk-num").textContent = n;
    pkPop.querySelector(".pk-head").textContent = label + " · SignupForm";
    pkPop.style.left = b.x + "px"; pkPop.style.top = b.y + b.h + 10 + "px";
    pkTa.classList.remove("typing"); pkTa.querySelector(".pk-tx").textContent = "";
    pkPop.classList.add("on");
    await wait(600);
    pkTa.classList.add("typing");
    for (var ch of note) { pkTa.querySelector(".pk-tx").textContent += ch; await wait(jitter(30, 60)); }
    await wait(400);
    await press(wait, pkPop.querySelector(".pk-add"));
    pkPop.classList.remove("on"); pkRing.classList.remove("on");
    var kept = pkRing.cloneNode(true);
    kept.classList.add("kept", "on");
    pkRing.before(kept);
    var rb = ring(kept, el), pin = pkPins[n - 1];
    // On the ring's straight top edge, past its rounded corner (notePin).
    pin.style.left = rb.x + 8 + "px"; pin.style.top = rb.y - 18 + "px";
    pin.classList.add("on");
    pkTray.querySelector(".pk-count").textContent = n === 1 ? "1 note" : n + " notes";
    pkTray.querySelector(".pk-rows").insertAdjacentHTML("beforeend", '<div class="pk-r"><span class="mono">' + label + "</span><span>" + note + "</span></div>");
    pkTray.classList.add("on");
    await wait(700);
  }
  async function pointAtPage(wait) {
    setCard(0); nameCard(BRANCH, signupPath, ""); setFooter(BRANCH, "claude", "opus 5.5 · medium", 0, 0);
    openBrowserTab();
    await wait(900);
    await press(wait, pick);
    bpane.classList.add("picking");
    hover(page.querySelector(".pg-nav"), "nav");
    await wait(700);
    await leaveNote(wait, heading, "h1", 1, "Lead with the trial: Start your free trial");
    await leaveNote(wait, lead, "p", 2, "Drop this line, the heading says it");
    await press(wait, pkTray.querySelector(".pk-send"));
    resetNotes();
    browserTerm.submit("[lich] The person left 2 notes on the page…");
    setRing(0, "busy");
    await browserTerm.stream(wait, ["● Update(src/SignupForm.tsx)", "  ⎿  " + bad("- &lt;h1>Create your account&lt;/h1>"), "  ⎿  " + ok("+ &lt;h1>Start your free trial&lt;/h1>"),
      "  ⎿  " + bad("- &lt;p>Start a 14-day trial. No card needed.&lt;/p>")]);
    setFooter(BRANCH, "claude", "opus 5.5 · medium", 1, 2);
    nameCard(BRANCH, signupPath, '<span class="add">+1</span><span class="del">-2</span>');
    browserTerm.lines.push("● lich - browser_reload (MCP)"); browserTerm.draw(true);
    var reload = brw.querySelector(".b-rel");
    glide(page, .5, .2); acur.classList.add("on");
    await wait(400);
    await land(wait, glide(reload));
    await flash(wait, reload);
    heading.textContent = "Start your free trial"; lead.hidden = true;
    await browserTerm.stream(wait, ["● The heading now leads with the trial and the line under it", "  is gone. Reloaded to check.",
      dim("✻ Crunched for 18s · done 21:07")]);
    setRing(0, "done");
    await wait(2600);
    await wait.end();
  }

  var stories = [startTask, mixAgents, reviewDiff, pullRequests, handOff, driveBrowser, pointAtPage];
  function start(v) {
    generation[v]++;
    told[v] = false; barDone = false;
    chip.classList.remove("on");
    resetTask();
    resetBrowser();
    wall.setAttribute("aria-label", labels[v]);
    stories[v](waiter(v)).catch(function () {});
  }

  wallTerms.forEach(function (t, i) { t.draw(i === 0); });
  openPr(0);
  terminal(hand[0].querySelector("pre"), "claude", [dim("✻ Crunched for 48s · done 20:24")]).draw(true);
  terminal(hand[1].querySelector("pre"), "codex", [dim("Worked for 22s • 8:33 PM")]).draw(false);
  if (still) return;

  // The open tab fills a bar under its label; the next tab opens once the bar is full and the story told.
  // Measured story lengths, rounded up; a story that runs longer still holds its tab until it is told.
  var durations = [17000, 16000, 22000, 10000, 16000, 21000, 20000];
  function runBar() {
    radios.forEach(function (r, i) {
      var label = seg.querySelector('[for="' + r.id + '"]');
      label.classList.remove("run");
      if (!r.checked) return;
      label.style.setProperty("--dur", durations[i] + "ms");
      void label.offsetWidth;
      label.classList.add("run");
    });
  }
  function sync() { seg.classList.toggle("paused", !shown); }
  seg.classList.add("auto");
  function advance() {
    if (still || !barDone || !told[active()]) return;
    radios[(active() + 1) % radios.length].checked = true;
    radios[active()].dispatchEvent(new Event("change"));
  }
  seg.addEventListener("animationend", function () { barDone = true; advance(); });
  radios.forEach(function (r, i) { r.addEventListener("change", function () { runBar(); start(i); }); });
  new IntersectionObserver(function (e) { shown = e[0].isIntersecting; sync(); }).observe(wall);
  runBar();
  start(0);
})();
