#!/usr/bin/env node
const fs = require('fs');
const path = require('path');
try {
  const [inp, out] = process.argv.slice(2);
  if (!inp || !out) throw new Error('usage: ua-tour-analyze.js <input> <output>');
  const { nodes = [], edges = [], layers = [] } = JSON.parse(fs.readFileSync(inp, 'utf8'));
  const byId = new Map(nodes.map(n => [n.id, n]));
  const fanIn = new Map(), fanOut = new Map(), adj = new Map();
  const inSet = new Map(), outSet = new Map();
  for (const n of nodes) { inSet.set(n.id, new Set()); outSet.set(n.id, new Set()); }
  for (const e of edges) {
    if (!byId.has(e.source) || !byId.has(e.target) || e.source === e.target) continue;
    outSet.get(e.source).add(e.target); inSet.get(e.target).add(e.source);
    if (e.type === 'imports' || e.type === 'calls') {
      if (!adj.has(e.source)) adj.set(e.source, new Set());
      adj.get(e.source).add(e.target);
    }
  }
  for (const n of nodes) { fanIn.set(n.id, inSet.get(n.id).size); fanOut.set(n.id, outSet.get(n.id).size); }
  const rank = (m, k) => [...m].sort((a, b) => b[1] - a[1]).slice(0, 20).map(([id, v]) => ({ id, [k]: v, name: byId.get(id).name }));
  const fanInRanking = rank(fanIn, 'fanIn'), fanOutRanking = rank(fanOut, 'fanOut');

  const pct = (arr, p) => { const s = [...arr].sort((a, b) => a - b); return s[Math.min(s.length - 1, Math.floor(p * s.length))]; };
  const outTop = pct([...fanOut.values()], 0.9), inLow = pct([...fanIn.values()], 0.25);
  const entryNames = new Set(['index.ts','index.js','main.ts','main.js','app.ts','app.js','server.ts','server.js','mod.rs','main.go','main.py','main.rs','manage.py','app.py','wsgi.py','asgi.py','run.py','__main__.py','Application.java','Main.java','Program.cs','config.ru','index.php','App.swift','Application.kt','main.cpp','main.c']);
  const docTypes = new Set(['document']);
  const cands = nodes.map(n => {
    const fp = n.filePath || n.name || ''; const depth = fp.split('/').length - 1; const base = path.basename(fp);
    let s = 0;
    if (docTypes.has(n.type)) {
      if (depth === 0 && /^readme\.md$/i.test(base)) s += 5; else if (depth === 0 && /\.md$/i.test(base)) s += 2;
    } else if (n.type === 'file') {
      if (entryNames.has(base)) s += 3;
      if (depth <= 1) s += 1;
      if (fanOut.get(n.id) >= outTop && outTop > 0) s += 1;
      if (fanIn.get(n.id) <= inLow) s += 1;
    }
    return { id: n.id, score: s, name: n.name, summary: n.summary, isDoc: docTypes.has(n.type) };
  }).filter(c => c.score > 0).sort((a, b) => b.score - a.score);
  const entryPointCandidates = cands.slice(0, 5).map(({ isDoc, ...c }) => c);

  const start = (cands.find(c => !c.isDoc && entryNames.has(path.basename(byId.get(c.id).filePath || ''))) || cands.find(c => !c.isDoc) || {}).id;
  const order = [], depthMap = {}, byDepth = {};
  if (start) {
    const q = [start]; depthMap[start] = 0;
    while (q.length) {
      const cur = q.shift(); order.push(cur); (byDepth[depthMap[cur]] ||= []).push(cur);
      for (const t of adj.get(cur) || []) if (!(t in depthMap)) { depthMap[t] = depthMap[cur] + 1; q.push(t); }
    }
  }

  const pick = n => ({ id: n.id, name: n.name, type: n.type, summary: n.summary });
  const cat = ts => nodes.filter(n => ts.includes(n.type)).map(pick);
  const nonCodeFiles = {
    documentation: cat(['document']), infrastructure: cat(['service', 'pipeline', 'resource']),
    data: cat(['table', 'schema', 'endpoint']), config: cat(['config']),
  };

  // clusters: bidirectional imports/calls pairs, expanded by nodes linked to 2+ members
  const has = (a, b) => adj.get(a)?.has(b);
  const und = (a, b) => outSet.get(a).has(b) || outSet.get(b).has(a);
  const seen = new Set(), clusters = [];
  for (const [a, ts] of adj) for (const b of ts) {
    if (a >= b || !has(b, a)) continue;
    const key = [a, b].sort().join('|'); if (seen.has(key)) continue; seen.add(key);
    const c = [a, b];
    for (const n of nodes) {
      if (c.length >= 5) break;
      if (c.includes(n.id)) continue;
      if (c.filter(m => und(m, n.id)).length >= 2) c.push(n.id);
    }
    let ec = 0; for (const x of c) for (const y of c) if (x !== y && outSet.get(x).has(y)) ec++;
    clusters.push({ nodes: c, edgeCount: ec });
  }
  clusters.sort((a, b) => b.edgeCount - a.edgeCount);

  const nodeSummaryIndex = {};
  for (const n of nodes) nodeSummaryIndex[n.id] = { name: n.name, type: n.type, summary: n.summary };
  fs.writeFileSync(out, JSON.stringify({
    scriptCompleted: true, entryPointCandidates, fanInRanking, fanOutRanking,
    bfsTraversal: { startNode: start || null, order, depthMap, byDepth }, nonCodeFiles,
    clusters: clusters.slice(0, 10), layers: { count: layers.length, list: layers },
    nodeSummaryIndex, totalNodes: nodes.length, totalEdges: edges.length,
  }, null, 2));
} catch (e) { console.error(e.stack || e.message); process.exit(1); }
