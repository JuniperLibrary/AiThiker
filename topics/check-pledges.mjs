#!/usr/bin/env node
/**
 * check-pledges.mjs — topics 承诺-存在矩阵（topics/SPEC.md §5.1 机器检查）
 *
 * 扫描 topics/ 下全部 *.md（AGENTS.md / index.md / SPEC.md / 内容文档）中出现的文件与目录引用
 * （Markdown 链接、反引号路径、表格"关键文档"承诺、正文路径），
 * 对照文件系统逐条判定存在性；同时检测行内控制字符（\t / \r）污染。
 *
 * 用法:  node topics/check-pledges.mjs            # 生成 topics/pledge-matrix.md
 * 退出码: 0 = 无缺失且无污染；1 = 存在缺失或污染
 */
import { readdirSync, readFileSync, existsSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const TOPICS_DIR = dirname(fileURLToPath(import.meta.url));
const ROOT = dirname(TOPICS_DIR);
const OUT_FILE = join(TOPICS_DIR, 'pledge-matrix.md');

const EXT = /\.(md|markdown|json|ya?ml|py|go|js|jsx|mjs|ts|tsx|html|sh|css|toml|txt)$/i;
const SKIP_TOKENS = ['target-dir', 'YYYY', 'brief-description', 'example', 'description'];

// 已确认的非文件承诺（约定性表述），不参与缺失判定
const IGNORE = new Set([
  'topics/AGENTS.md::index.md', // 约定："每个子目录以 index.md 为入口"，非具体文件承诺
  'topics/SPEC.md::index.md', // 表述：指各子目录的 index.md，非具体文件承诺
]);
// 已知缺失项的备注（不豁免，仅说明）
const NOTES = {
  'resources/papers.md': 'SPEC 待核清单 #6：补建或删除引用',
};

const rel = (p) => relative(ROOT, p).split(sep).join('/');

// 外部系统路径白名单（BondSeek 源码目录 / 示例路径，不在本仓库，存在性不可判定）
const EXTERNAL_ROOTS = new Set([
  'bondseek-agent', 'bondseek-common', 'agenttools', 'core', 'retriever', 'tools',
  'context', 'aistudio', 'agentcore', 'session', 'project', 'api',
  '.local', '.config', '.aithiker',
]);

// 全仓库 .md 文件 basename 索引（用于识别"文档名指称"）
function* walkMd(dir) {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name === 'node_modules' || e.name === '.git') continue;
      yield* walkMd(p);
    } else if (e.name.endsWith('.md')) yield p;
  }
}
const basenameIndex = new Map();
for (const f of walkMd(ROOT)) {
  const b = f.split(sep).pop();
  if (!basenameIndex.has(b)) basenameIndex.set(b, []);
  basenameIndex.get(b).push(rel(f));
}

function isPathLike(s) {
  if (!s || s.includes('*')) return false;
  if (SKIP_TOKENS.some((t) => s.includes(t))) return false;
  if (!/^[\w./#-]+$/.test(s)) return false; // 含空格/中文等不算路径
  return EXT.test(s) || s.endsWith('/');
}

function* walk(dir) {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) yield* walk(p);
    else if (e.name.endsWith('.md') && p !== OUT_FILE) yield p;
  }
}

const refs = new Map(); // key: `${file}::${ref}` -> { line, kinds:Set, corrupted, stub }
const corruption = []; // { file, line, snippet }

function addRef(file, ref, kind, line, corrupted, stub) {
  ref = ref.split('#')[0].trim();
  if (!ref || !isPathLike(ref)) return;
  const key = `${file}::${ref}`;
  if (IGNORE.has(key)) return;
  if (!refs.has(key)) refs.set(key, { line, kinds: new Set(), corrupted, stub });
  const r = refs.get(key);
  r.kinds.add(kind);
  if (stub) r.stub = true;
}

const isRow = (l) => /^\s*\|.*\|\s*$/.test(l);
const isSep = (l) => /^\s*\|?[\s:|-]+\|?\s*$/.test(l) && l.includes('-');
const splitRow = (l) => l.trim().replace(/^\|/, '').replace(/\|$/, '').split('|');

const files = [...walk(TOPICS_DIR)].sort();

for (const file of files) {
  const relF = rel(file);
  const raw = readFileSync(file, 'utf8').replace(/^﻿/, '');
  const lines = raw.split('\n');
  let inFence = false;
  let inTable = false;
  let headerSkip = [];

  for (let i = 0; i < lines.length; i++) {
    const lineNo = i + 1;
    const noTrailCR = lines[i].replace(/\r$/, '');
    const corrupted = /[\x00-\x09\x0B-\x1F]/.test(noTrailCR);
    if (corrupted) {
      corruption.push({
        file: relF,
        line: lineNo,
        snippet: JSON.stringify(noTrailCR.trim()).slice(0, 100),
      });
    }
    const line = noTrailCR.replace(/[\x00-\x09\x0B-\x1F]/g, '');

    const trimmed = line.trimStart();
    if (trimmed.startsWith('```') || trimmed.startsWith('~~~')) {
      inFence = !inFence;
      continue;
    }
    if (inFence) continue;

    // —— 表格解析：识别表头，跳过"示例/命名模式"列 ——
    if (isRow(line) && i + 1 < lines.length && isSep(lines[i + 1])) {
      const headerCells = splitRow(line.replace(/[\t\r]/g, ''));
      headerSkip = headerCells.map((h) => /示例|命名模式/.test(h));
      inTable = true;
      continue;
    }
    if (isSep(line)) continue;
    if (inTable && !isRow(line)) {
      inTable = false;
      headerSkip = [];
    }

    const text = line.replace(/https?:\/\/\S+/g, ' ');
    const stub = /计划|未创建|待创建|stub/i.test(line); // 显式声明"未创建"的行，其引用归为 stub 承诺

    // —— Markdown 链接 ——
    for (const m of text.matchAll(/\[[^\]]*\]\(([^)\s]+)\)/g)) {
      const t = m[1];
      if (/^(https?:|mailto:)/i.test(t) || t.startsWith('#')) continue;
      addRef(relF, t, '链接', lineNo, corrupted, stub);
    }
    // 链接整体（含链接文字与目标）不再参与反引号/正文提取，避免把链接文字误当引用；
    // 必须在剥离裸 URL 之前剥离完整链接语法，否则 [text](http://...) 会退化成 [text] 残留链接文字
    const noLinks = line.replace(/\[[^\]]*\]\([^)]+\)/g, ' ').replace(/https?:\/\/\S+/g, ' ');
    // —— 反引号路径 ——
    for (const m of noLinks.matchAll(/`([^`]+)`/g)) {
      addRef(relF, m[1].trim(), '反引号', lineNo, corrupted, stub);
    }
    // —— 表格承诺（裸文件名/路径单元格） ——
    if (inTable) {
      const cells = splitRow(line);
      cells.forEach((c, idx) => {
        if (headerSkip[idx]) return;
        const cell = c.trim();
        if (!cell || /\s/.test(cell) || cell.includes('](')) return;
        addRef(relF, cell, '表格', lineNo, corrupted, stub);
      });
    }
    // —— 正文中的斜杠路径（须以 / 结尾或带扩展名，滤掉 p50/p95、BFS/DFS） ——
    for (const m of noLinks.matchAll(/[A-Za-z0-9_.-]+(?:\/[A-Za-z0-9_.-]+)+\/?/g)) {
      addRef(relF, m[0], '正文', lineNo, corrupted, stub);
    }
  }
}

// —— 存在性判定 ——
function resolveRef(fromFileRel, ref) {
  const fromAbs = join(ROOT, fromFileRel);
  const attempts = [];
  const cands = [];
  if (ref.startsWith('./') || ref.startsWith('../')) {
    cands.push(resolve(dirname(fromAbs), ref));
  } else if (ref.includes('/')) {
    cands.push(resolve(dirname(fromAbs), ref));
    cands.push(resolve(TOPICS_DIR, ref));
    cands.push(resolve(ROOT, ref));
  } else {
    // 裸文件名：本目录 → topics/ → 仓库根
    cands.push(resolve(dirname(fromAbs), ref));
    cands.push(resolve(TOPICS_DIR, ref));
    cands.push(resolve(ROOT, ref));
  }
  for (const abs of [...new Set(cands)]) {
    attempts.push(rel(abs));
    if (existsSync(abs)) return { ok: true, at: rel(abs) };
  }
  return { ok: false, attempts };
}

const missing = [];
const ok = [];
const stubbed = []; // ⚪ 显式未创建承诺：行内已声明"计划/未创建/待创建/stub"
const nameRefs = []; // ⚪ 文档名指称：裸文件名提及，可唯一解析到仓库文件
const external = []; // ⚪ 外部系统引用：首段命中白名单，机器无法判定
for (const [key, meta] of [...refs.entries()].sort()) {
  const [file, ref] = key.split('::');
  const res = resolveRef(file, ref);
  const row = { file, line: meta.line, ref, kinds: [...meta.kinds].join('+'), corrupted: meta.corrupted, ...res };
  if (res.ok) { ok.push(row); continue; }
  if (meta.stub) { stubbed.push(row); continue; }
  const firstSeg = ref.replace(/^\.\//, '').split('/')[0];
  if (EXTERNAL_ROOTS.has(firstSeg)) { external.push(row); continue; }
  if (!ref.includes('/')) {
    const hits = basenameIndex.get(ref);
    if (hits && hits.length === 1) { nameRefs.push({ ...row, nameTarget: hits[0] }); continue; }
  }
  missing.push(row);
}

// —— 生成报告 ——
const date = new Date().toISOString().slice(0, 10);
const esc = (s) => s.replace(/\|/g, '\\|');
let out = `# topics 承诺-存在矩阵报告

> **生成日期:** ${date}
> **生成方式:** \`node topics/check-pledges.mjs\`（机器判定，可重跑）
> **范围:** topics/ 下全部 *.md（${files.length} 个文件，不含本报告）
> **判定规则:** 文件中出现的文件/目录引用（Markdown 链接、反引号路径、表格承诺、正文路径）必须在文件系统中存在；含斜杠路径依次按本目录 → topics/ → 仓库根解析，裸文件名同序解析。首段命中外部系统白名单（BondSeek 源码/示例路径）的标记 ⚪ 不可判定；裸文件名在仓库中有唯一同名文件的标记 ⚪ 文档名指称。
> **对应题目:** topics/SPEC.md §5.1（深度轴 D6 对抗验证之机器裁决）

---

## 汇总

| 指标 | 数值 |
|---|---|
| 扫描文件 | ${files.length} |
| 引用总数 | ${refs.size} |
| ✅ 存在 | ${ok.length} |
| ❌ 缺失 | ${missing.length} |
| ⚪ 文档名指称（未链接，可唯一解析） | ${nameRefs.length} |
| ⚪ 外部系统引用（机器不可判定） | ${external.length} |
| ⚪ 显式未创建承诺（stub） | ${stubbed.length} |
| ⚠️ 控制字符污染（\\t / \\r 混入正文） | ${corruption.length} 处 / ${new Set(corruption.map((c) => c.file)).size} 个文件 |

---

## ❌ 缺失（${missing.length}）—— 必须补齐或删除承诺

| 来源文件 | 行 | 引用 | 判定尝试 | 备注 |
|---|---|---|---|---|
${missing.map((r) => `| ${r.file} | ${r.line} | \`${esc(r.ref)}\` | ${r.attempts.map(esc).join(' / ')} | ${NOTES[r.ref] ?? ''}${r.corrupted ? '原行含控制字符' : ''} |`).join('\n')}

---

## ⚪ 显式未创建承诺（stub，${stubbed.length}）—— 行内已声明"计划/未创建/待创建"，不视为坏承诺；补齐文档后移出本表

| 来源文件 | 行 | 引用 |
|---|---|---|
${stubbed.map((r) => `| ${r.file} | ${r.line} | \`${esc(r.ref)}\` |`).join('\n')}

---

## ⚪ 文档名指称（${nameRefs.length}）—— 裸文件名提及，可唯一解析到仓库文件；建议按约定转为相对链接

| 来源文件 | 行 | 引用 | 唯一匹配 |
|---|---|---|---|
${nameRefs.map((r) => `| ${r.file} | ${r.line} | \`${esc(r.ref)}\` | ${esc(r.nameTarget)} |`).join('\n')}

---

## ⚪ 外部系统引用（${external.length}）—— 指向 BondSeek 源码 / 示例路径，不在本仓库，存在性机器不可判定（SPEC §9 待核 #1）

| 来源文件 | 行 | 引用 |
|---|---|---|
${external.map((r) => `| ${r.file} | ${r.line} | \`${esc(r.ref)}\` |`).join('\n')}

---

## ⚠️ 行内控制字符污染（${corruption.length}）—— 正文中混入字面 \\t / \\r

| 文件 | 行 | 片段 |
|---|---|---|
${corruption.map((c) => `| ${c.file} | ${c.line} | ${c.snippet} |`).join('\n')}

---

## ✅ 存在（${ok.length}）

| 来源文件 | 行 | 引用 | 解析到 |
|---|---|---|---|
${ok.map((r) => `| ${r.file} | ${r.line} | \`${esc(r.ref)}\` | ${esc(r.at)} |`).join('\n')}

---

> 本报告由脚本生成，禁止手工修改；修正问题后重跑 \`node topics/check-pledges.mjs\` 更新。
> 验收标准（SPEC §8）：❌ 缺失 = 0 且 ⚠️ 污染 = 0。
`;

writeFileSync(OUT_FILE, out, 'utf8');

console.log(`扫描 ${files.length} 个文件，引用 ${refs.size} 条：✅ ${ok.length} / ❌ ${missing.length}；控制字符污染 ${corruption.length} 处`);
console.log(`报告已写入 ${rel(OUT_FILE)}`);
process.exit(missing.length > 0 || corruption.length > 0 ? 1 : 0);
