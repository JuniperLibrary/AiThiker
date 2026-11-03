/* 纯逻辑单元测试(无依赖,直接运行):
 *   node test-logic.js
 * 测试用例从 index.html 的「纯逻辑区」抽取,不依赖 DOM。
 */
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
const m = html.match(/纯逻辑:开始 ===== \*\/([\s\S]*?)\/\* ===== 纯逻辑:结束/);
if (!m) throw new Error('未找到纯逻辑区段');

const sandbox = { module: { exports: {} }, console };
vm.createContext(sandbox);
vm.runInContext(m[1], sandbox);
const L = sandbox.module.exports;

let pass = 0, fail = 0;
function eq(name, actual, expected) {
  const a = JSON.stringify(actual), e = JSON.stringify(expected);
  if (a === e) { pass++; console.log('  ok  ' + name); }
  else {
    fail++;
    console.log('  FAIL ' + name + '\n       actual=' + a + '\n       expect=' + e);
  }
}
function ok(name, cond) { eq(name, !!cond, true); }

const D = (s) => new Date(s);
const base = { records: [], limit: 12, override: 'auto', now: D('2026-11-02T07:00:00') };

console.log('\n[区域分类]');
eq('天安门 -> five', L.classifyZone(39.9087, 116.3975).zone, 'five');
eq('国贸 -> five', L.classifyZone(39.9087, 116.4612).zone, 'five');
eq('通州万达 -> tong', L.classifyZone(39.9060, 116.6460).zone, 'tong');
eq('大兴机场 -> outside', L.classifyZone(39.5086, 116.4109).zone, 'outside');
eq('首都机场T3 -> six', L.classifyZone(40.0527, 116.6114).zone, 'six');

console.log('\n[工作日 / 节假日 / 高峰]');
eq('周一 08:00 早高峰', L.peakOf(D('2026-11-02T08:00:00'), 'auto').key, 'am');
eq('周一 10:00 非高峰', L.peakOf(D('2026-11-02T10:00:00'), 'auto'), null);
eq('周一 18:00 晚高峰', L.peakOf(D('2026-11-02T18:00:00'), 'auto').key, 'pm');
eq('周一 20:00 已结束', L.peakOf(D('2026-11-02T20:00:00'), 'auto'), null);
eq('周日 08:00 不限行', L.peakOf(D('2026-11-01T08:00:00'), 'auto'), null);
eq('国庆 10-01 08:00 不限行', L.peakOf(D('2026-10-01T08:00:00'), 'auto'), null);
eq('补班 10-10(周六)08:00 早高峰', L.peakOf(D('2026-10-10T08:00:00'), 'auto').key, 'am');
eq('补班手动指定', L.peakOf(D('2026-11-01T08:00:00'), 'workday').key, 'am');
eq('节假日手动指定', L.peakOf(D('2026-11-02T08:00:00'), 'holiday'), null);
eq('下个可通行: 08:30 -> 09:00', L.hm(L.nextAllowedTime(D('2026-11-02T08:30:00'), 'auto')), '09:00');
eq('下个可通行: 18:30 -> 20:00', L.hm(L.nextAllowedTime(D('2026-11-02T18:30:00'), 'auto')), '20:00');

console.log('\n[进京证日期边界]');
const rec = { date: '2026-11-01', days: 7 };
eq('7 天覆盖 11-07', L.coversDate(rec, D('2026-11-07T12:00:00')), true);
eq('7 天不覆盖 11-08', L.coversDate(rec, D('2026-11-08T12:00:00')), false);
eq('到期日为 11-07', L.ymd(L.recordEnd(rec)), '2026-11-07');
eq('计划日证件状态', L.permitState([rec], D('2026-11-07T08:00:00'), 12).state, 'valid');
eq('计划日证件到期日', L.permitState([rec], D('2026-11-07T08:00:00'), 12).end, '2026-11-07');
eq('11-08 状态', L.permitState([rec], D('2026-11-08T08:00:00'), 12).state, 'none');
eq('跨年记录按起始年计数', L.countYear([{ date: '2026-12-28', days: 7 }], 2026), 1);
eq('记录重叠', L.rangesOverlap({ date: '2026-11-01', days: 7 }, { date: '2026-11-07', days: 7 }), true);
eq('记录不重叠', L.rangesOverlap({ date: '2026-11-01', days: 7 }, { date: '2026-11-08', days: 7 }), false);

console.log('\n[合规结论]');
let r = L.analyze(39.9087, 116.3975, D('2026-11-02T08:00:00'), base);
eq('五环内 周一早高峰 -> bad', r.verdict, 'bad');
eq('五环内 早高峰 -> 09:00 可通行', L.hm(r.nextAllowed), '09:00');
eq('五环内 早高峰仍需进京证', r.needPerm, true);

r = L.analyze(39.9087, 116.3975, D('2026-11-02T10:00:00'), base);
eq('五环内 非高峰 无证 -> permit', r.verdict, 'permit');
eq('五环内 无证 -> 状态 none', r.permit.state, 'none');

r = L.analyze(39.9087, 116.3975, D('2026-11-02T10:00:00'),
  { ...base, records: [{ date: '2026-11-01', days: 7 }] });
eq('五环内 非高峰 有证 -> ok', r.verdict, 'ok');
ok('有证标题包含「持有效进京证」', r.title.indexOf('持有效进京证') >= 0);

r = L.analyze(39.9087, 116.3975, D('2026-11-01T10:00:00'), base);
eq('周日 五环内 -> permit(不高峰)', r.verdict, 'permit');
eq('周日 无高峰', r.peak, null);

r = L.analyze(39.9087, 116.3975, D('2026-10-01T08:00:00'), base);
eq('国庆 五环内 08:00 不高峰 -> permit', r.verdict, 'permit');

r = L.analyze(39.9087, 116.3975, D('2026-10-10T08:00:00'), base);
eq('补班日 08:00 -> bad', r.verdict, 'bad');

r = L.analyze(39.9060, 116.6460, D('2026-11-02T08:00:00'), base);
eq('通州 早高峰 -> permit(不受五环高峰)', r.verdict, 'permit');
eq('通州 需进京证', r.needPerm, true);

r = L.analyze(39.5086, 116.4109, D('2026-11-02T08:00:00'), base);
eq('大兴机场 -> ok', r.verdict, 'ok');
eq('大兴机场 无需进京证', r.needPerm, false);

r = L.analyze(39.9087, 116.3975, D('2026-11-02T10:00:00'),
  { ...base, limit: 1, records: [{ date: '2026-01-01', days: 7 }] });
eq('次数用完 -> bad', r.verdict, 'bad');
eq('次数用完 状态', r.permit.state, 'exhausted');

r = L.analyze(39.9087, 116.3975, D('2027-03-01T08:00:00'), base);
eq('未知年份按周一工作日判断 -> bad', r.verdict, 'bad');
eq('未知年份日历标记', L.hasCalendarYear(2027), false);
eq('2026 日历标记', L.hasCalendarYear(2026), true);

console.log('\n[几何]');
ok('五环内点距边界 > 0', L.distToPoly(L.POLY_FIVE, 39.9087, 116.3975) > 0);
ok('天安门在五环内', L.pointInPoly(L.POLY_FIVE, 39.9087, 116.3975));
ok('大兴机场不在六环内', !L.pointInPoly(L.POLY_SIX, 39.5086, 116.4109));

console.log('\n[坐标系转换]');
function approxKm(a, b) {
  const lat = (a[0] + b[0]) / 2 * Math.PI / 180;
  return Math.hypot((a[1] - b[1]) * 111.320 * Math.cos(lat), (a[0] - b[0]) * 110.574);
}
const tian = [39.9087, 116.3975];
const tianG = L.wgs2gcj(tian[0], tian[1]);
const shiftKm = approxKm(tian, tianG);
ok('WGS->GCJ 偏移在 0.1–1 km 之间(' + shiftKm.toFixed(3) + ' km)', shiftKm > 0.1 && shiftKm < 1);
const back = L.gcj2wgs(tianG[0], tianG[1]);
ok('GCJ->WGS 往返误差 < 1 m(' + (approxKm(tian, back) * 1000).toFixed(2) + ' m)', approxKm(tian, back) < 0.001);
ok('往返后仍判为五环内', L.classifyZone(back[0], back[1]).zone === 'five');
ok('境外坐标不偏移', L.wgs2gcj(48.8566, 2.3522)[0] === 48.8566 && L.gcj2wgs(48.8566, 2.3522)[1] === 2.3522);

console.log('\n[路线体检]');
const isDate = (x) => x && typeof x.getTime === 'function' && !isNaN(x.getTime());
const routeVerts = [
  { lat: 39.70, lng: 116.30, t: 0 },
  { lat: 39.9087, lng: 116.3975, t: 1800 }
];
const rBase = { records: [], limit: 12, override: 'auto' };

let rr = L.analyzeRoute(routeVerts, D('2026-11-02T07:40:00'), rBase);
eq('早高峰进五环 -> bad', rr.verdict, 'bad');
ok('识别到高峰冲突', rr.peakConflict);
ok('问题列表非空', rr.issues.length > 0);
ok('给出最早可避开高峰的出发时间', isDate(rr.earliestWithPermit));

rr = L.analyzeRoute(routeVerts, D('2026-11-02T10:00:00'), rBase);
eq('非高峰无证 -> permit', rr.verdict, 'permit');
ok('路线需进京证', rr.needPerm);

rr = L.analyzeRoute(routeVerts, D('2026-11-02T10:00:00'),
  { ...rBase, records: [{ date: '2026-11-02', days: 2 }] });
eq('非高峰有证 -> ok', rr.verdict, 'ok');

rr = L.analyzeRoute([
  { lat: 39.60, lng: 116.30, t: 0 },
  { lat: 39.62, lng: 116.32, t: 600 }
], D('2026-11-02T08:00:00'), rBase);
eq('全程六环外 -> ok', rr.verdict, 'ok');
ok('六环外无需进京证', !rr.needPerm);

rr = L.analyzeRoute(routeVerts, D('2026-11-02T07:40:00'),
  { ...rBase, records: [{ date: '2026-11-02', days: 2 }] });
ok('持证时可求最早合法出发', isDate(rr.earliestLegal));
const rerun = L.analyzeRoute(routeVerts, rr.earliestLegal,
  { ...rBase, records: [{ date: '2026-11-02', days: 2 }] });
eq('改到最早合法时间 -> ok', rerun.verdict, 'ok');

rr = L.analyzeRoute([
  { lat: 39.60, lng: 116.30, t: 0 },
  { lat: 39.9087, lng: 116.3975, t: 30000 }
], D('2026-11-02T20:00:00'), { ...rBase, records: [{ date: '2026-11-02', days: 1 }] });
eq('跨天证件过期 -> permit', rr.verdict, 'permit');

console.log('\n结果: ' + pass + ' passed, ' + fail + ' failed');
if (fail) process.exit(1);
