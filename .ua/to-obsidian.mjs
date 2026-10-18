#!/usr/bin/env node
// knowledge-graph.json -> Obsidian notes + JSON Canvas.
// usage: node to-obsidian.mjs <knowledge-graph.json> <vault-dir> [folder]
import fs from 'node:fs';
import path from 'node:path';

const [graphPath, vault, folder = 'devscope'] = process.argv.slice(2);
if (!graphPath || !vault) { console.error('usage: to-obsidian.mjs <graph.json> <vault> [folder]'); process.exit(1); }
const g = JSON.parse(fs.readFileSync(graphPath, 'utf8'));
const root = path.join(vault, folder);

const FILE_TYPES = new Set(['file', 'config', 'document', 'service', 'pipeline', 'table', 'schema', 'resource', 'endpoint']);
const byId = new Map(g.nodes.map(n => [n.id, n]));
const files = g.nodes.filter(n => FILE_TYPES.has(n.type) && n.filePath);
const fileOf = id => byId.get(id)?.filePath; // any node -> its file
const notePath = fp => `${folder}/${fp}`; // vault-relative, without .md
const link = fp => `[[${notePath(fp)}|${path.basename(fp)}]]`;

// Roll every edge up to file level: srcFile -> tgtFile -> Set(types)
const fileEdges = new Map();
for (const e of g.edges) {
  const s = fileOf(e.source), t = fileOf(e.target);
  if (!s || !t || s === t || e.type === 'contains') continue;
  const k = `${s}\0${t}`;
  if (!fileEdges.has(k)) fileEdges.set(k, new Set());
  fileEdges.get(k).add(e.type);
}

const layerOf = new Map();
for (const l of g.layers || []) for (const id of l.nodeIds) layerOf.set(byId.get(id)?.filePath, l);

// --- Notes ---
fs.rmSync(root, { recursive: true, force: true });
const children = new Map(); // file -> symbol nodes
for (const n of g.nodes) if (!FILE_TYPES.has(n.type) && n.filePath) {
  if (!children.has(n.filePath)) children.set(n.filePath, []);
  children.get(n.filePath).push(n);
}
const out = new Map(), inc = new Map();
for (const [k, types] of fileEdges) {
  const [s, t] = k.split('\0');
  (out.get(s) || out.set(s, []).get(s)).push([t, types]);
  (inc.get(t) || inc.set(t, []).get(t)).push([s, types]);
}
const yamlList = a => `[${a.map(x => JSON.stringify(String(x).replace(/\s+/g, '-'))).join(', ')}]`;

for (const n of files) {
  const fp = n.filePath, l = layerOf.get(fp);
  const md = [
    '---',
    `type: ${n.type}`,
    `path: ${JSON.stringify(fp)}`,
    l ? `layer: ${JSON.stringify(l.name)}` : null,
    n.complexity ? `complexity: ${n.complexity}` : null,
    `tags: ${yamlList(n.tags || [])}`,
    '---',
    `# ${n.name}`, '', n.summary || '', '',
    l ? `**Camada:** [[${folder}/_camadas/${l.name.replace(/[\\/:]/g, '-')}|${l.name}]]\n` : null,
  ];
  const syms = children.get(fp) || [];
  if (syms.length) {
    md.push('## Símbolos', '');
    for (const s of syms) md.push(`- **${s.name}** _(${s.type})_ — ${s.summary || ''}`);
    md.push('');
  }
  // Go imports resolve per package (every file of the package), so list packages, not file links.
  const sec = (title, list) => {
    const semantic = (list || []).map(([f, t]) => [f, [...t].filter(x => x !== 'imports')]).filter(([, t]) => t.length);
    const pkgs = [...new Set((list || []).filter(([, t]) => t.has('imports')).map(([f]) => path.dirname(f)))].sort();
    if (!semantic.length && !pkgs.length) return;
    md.push(`## ${title}`, '');
    for (const [f, t] of semantic) md.push(`- ${link(f)} — ${t.join(', ')}`);
    if (pkgs.length) md.push(`- _imports (pacotes):_ ${pkgs.map(d => '`' + d + '`').join(', ')}`);
    md.push('');
  };
  sec('Depende de', out.get(fp));
  sec('Usado por', inc.get(fp));
  if (n.languageNotes) md.push('## Notas', '', n.languageNotes, '');
  const dest = path.join(vault, notePath(fp) + '.md');
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  fs.writeFileSync(dest, md.filter(x => x !== null).join('\n'));
}

// Layer notes + tour note
fs.mkdirSync(path.join(root, '_camadas'), { recursive: true });
for (const l of g.layers || []) {
  const fps = l.nodeIds.map(fileOf).filter(Boolean);
  fs.writeFileSync(path.join(root, '_camadas', l.name.replace(/[\\/:]/g, '-') + '.md'),
    [`# ${l.name}`, '', l.description, '', ...fps.map(f => `- ${link(f)}`)].join('\n'));
}
if (g.tour?.length) fs.writeFileSync(path.join(root, '00 - Tour.md'), [
  `# Tour: ${g.project.name}`, '', g.project.description, '',
  ...g.tour.flatMap(s => [`## ${s.order}. ${s.title}`, '', s.description, '',
    ...s.nodeIds.map(fileOf).filter(Boolean).map(f => `- ${link(f)}`), '',
    s.languageLesson ? `> ${s.languageLesson}\n` : '']),
].join('\n'));

// --- Canvas ---
// Detailed canvas: one group per layer, one file card per file, file-level edges.
// ponytail: grid layout, no force-directed placement; rearrange in Obsidian if needed.
const W = 400, H = 220, GAP = 60, PAD = 60, HEADER = 40;
const nodes = [], edges = [], cardId = new Map();
const layers = (g.layers || []).map(l => ({ l, fps: [...new Set(l.nodeIds.map(fileOf).filter(Boolean))] }));
const orphans = files.map(n => n.filePath).filter(fp => !layerOf.has(fp));
if (orphans.length) layers.push({ l: { id: 'layer:outros', name: 'Outros' }, fps: orphans });

const groupCols = Math.ceil(Math.sqrt(layers.length));
let x = 0, y = 0, rowH = 0, col = 0;
const COLORS = ['1', '2', '3', '4', '5', '6'];
layers.forEach(({ l, fps }, i) => {
  const cols = Math.max(1, Math.ceil(Math.sqrt(fps.length)));
  const rows = Math.ceil(fps.length / cols);
  const gw = cols * (W + GAP) - GAP + 2 * PAD, gh = rows * (H + GAP) - GAP + 2 * PAD + HEADER;
  const color = COLORS[i % COLORS.length];
  nodes.push({ id: `g${i}`, type: 'group', label: l.name, x, y, width: gw, height: gh, color });
  fps.forEach((fp, j) => {
    const id = `n${cardId.size}`;
    cardId.set(fp, id);
    nodes.push({ id, type: 'file', file: notePath(fp) + '.md', color,
      x: x + PAD + (j % cols) * (W + GAP), y: y + PAD + HEADER + Math.floor(j / cols) * (H + GAP), width: W, height: H });
  });
  rowH = Math.max(rowH, gh); x += gw + 4 * GAP;
  if (++col === groupCols) { col = 0; x = 0; y += rowH + 4 * GAP; rowH = 0; }
});
let ei = 0;
for (const [k, types] of fileEdges) {
  const [s, t] = k.split('\0');
  const label = [...types].filter(x => x !== 'imports'); // imports are per-package noise on a canvas
  if (label.length && cardId.has(s) && cardId.has(t))
    edges.push({ id: `e${ei++}`, fromNode: cardId.get(s), toNode: cardId.get(t), toEnd: 'arrow', label: label.join(', ') });
}
fs.writeFileSync(path.join(root, `${folder}.canvas`), JSON.stringify({ nodes, edges }, null, 1));

// Overview canvas: one card per layer, edges aggregated between layers (readable at a glance).
const lname = fp => layerOf.get(fp)?.name ?? 'Outros';
const agg = new Map();
for (const k of fileEdges.keys()) {
  const [s, t] = k.split('\0'), a = lname(s), b = lname(t);
  if (a !== b) agg.set(`${a}\0${b}`, (agg.get(`${a}\0${b}`) || 0) + 1);
}
const ov = layers.map(({ l, fps }, i) => ({
  id: `L${i}`, type: 'text', color: COLORS[i % COLORS.length],
  text: `## [[${folder}/_camadas/${l.name.replace(/[\\/:]/g, '-')}|${l.name}]]\n${l.description || ''}\n\n_${fps.length} arquivos_`,
  x: (i % groupCols) * 700, y: Math.floor(i / groupCols) * 500, width: 520, height: 320,
}));
const ovId = new Map(layers.map(({ l }, i) => [l.name, `L${i}`]));
const ovEdges = [...agg].map(([k, c], i) => {
  const [a, b] = k.split('\0');
  return { id: `E${i}`, fromNode: ovId.get(a), toNode: ovId.get(b), toEnd: 'arrow', label: `${c} ligações` };
});
fs.writeFileSync(path.join(root, `${folder} - visão geral.canvas`), JSON.stringify({ nodes: ov, edges: ovEdges }, null, 1));

console.log(`notes=${files.length} layers=${layers.length} canvasCards=${cardId.size} canvasEdges=${edges.length} overviewEdges=${ovEdges.length} -> ${root}`);
