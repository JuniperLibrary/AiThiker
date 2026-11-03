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
eq('补班 10-10(周六)08:00 不限行(通告括注)', L.peakOf(D('2026-10-10T08:00:00'), 'auto'), null);
eq('补班手动指定 -> 不限行', L.peakOf(D('2026-11-01T08:00:00'), 'workday'), null);
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
eq('补班日 08:00 -> permit(不限行,仅需证)', r.verdict, 'permit');
eq('补班日无高峰', r.peak, null);

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

console.log('\n[规则库 schema(法源驱动)]');
const schemaErrs = [];
const seenIds = new Set();
L.RULES.forEach((r) => {
  if (!r.id) schemaErrs.push('缺 id');
  else if (seenIds.has(r.id)) schemaErrs.push(r.id + ': id 重复');
  else seenIds.add(r.id);
  ['title', 'confidence', 'implemented'].forEach((k) => {
    if (r[k] === undefined || r[k] === '') schemaErrs.push((r.id || '?') + ': 缺 ' + k);
  });
  if (!['high', 'medium', 'low'].includes(r.confidence)) schemaErrs.push(r.id + ': confidence 非法');
  const s = r.source || {};
  ['file', 'url', 'verified_at'].forEach((k) => {
    if (!s[k]) schemaErrs.push((r.id || '?') + ': source.' + k + ' 缺失');
  });
  if (!r.elements) schemaErrs.push((r.id || '?') + ': 缺 elements');
  if (r.implemented === false && !r.gap) schemaErrs.push(r.id + ': 未覆盖但无 gap 说明');
});
eq('schema 校验无错误', schemaErrs, []);
eq('规则数 >= 10', L.RULES.length >= 10, true);
eq('PEAK_AM 从规则派生', L.getRule('peak-five-ring').elements.peak_am, [420, 540]);
eq('PEAK_PM 从规则派生', L.getRule('peak-five-ring').elements.peak_pm, [1020, 1200]);
eq('年度次数从规则派生', L.PERMIT_YEAR_LIMIT, 12);
eq('单次天数从规则派生', L.PERMIT_MAX_DAYS, 7);
ok('高峰处罚已核实(100 元/记 1 分)', /100 元/.test(L.getRule('peak-five-ring').penalty.detail) &&
  /记 1 分/.test(L.getRule('peak-five-ring').penalty.detail));
ok('外埠新能源不豁免已核实',
  L.getRule('exempt-vehicles').elements.exceptions.join('').includes('外省、区、市号牌新能源小客车'));
eq('未覆盖规则带缺口说明',
  L.RULES.filter((r) => r.implemented === false && !r.gap).length, 0);

console.log('\n[尾号限行(9-17 时)]');
eq('2026-11-03 周二 10:00 限行尾号组', L.tailGroupFor(D('2026-11-03T10:00:00'), 'auto'), [1, 6]);
eq('2026-11-02 周一 10:00 限行尾号组', L.tailGroupFor(D('2026-11-02T10:00:00'), 'auto'), [5, 0]);
eq('周六不适用', L.tailApplicable(D('2026-11-07T10:00:00'), 'auto'), false);
eq('08:59 属高峰窗口,尾号组不适用(9 点起)', L.tailGroupFor(D('2026-11-03T08:59:00'), 'auto'), null);
eq('16:59 适用', L.tailGroupFor(D('2026-11-03T16:59:00'), 'auto'), [1, 6]);
eq('17:00 起为高峰窗口', L.tailGroupFor(D('2026-11-03T17:00:00'), 'auto'), null);
eq('字母尾号归一为 0', L.normalizePlate('京A12345'.toUpperCase()), 5);
eq('单字母归一为 0', L.normalizePlate('A'), 0);
eq('数字尾号', L.normalizePlate('6'), 6);
eq('空尾号', L.normalizePlate(''), null);
eq('轮换表外日期 -> null', L.tailGroupFor(D('2027-04-01T10:00:00'), 'auto'), null);
const tv = L.tailViolation(D('2026-11-03T10:00:00'), 'auto', '6');
eq('周二尾号 6 命中限行', tv.restricted, true);
eq('未填尾号 -> known:false', L.tailViolation(D('2026-11-03T10:00:00'), 'auto', '').known, false);

console.log('\n[黄金测试集(来源标注)]');
const gbase = { records: [], limit: 12, override: 'auto', now: D('2026-11-03T07:00:00') };
const permitRec = [{ date: '2026-01-25', days: 7 }];
const octRec = [{ date: '2026-10-08', days: 7 }];
const octRec2 = [{ date: '2026-10-01', days: 7 }];
const novRec = [{ date: '2026-11-02', days: 3 }];
const novRec2 = [{ date: '2026-11-06', days: 3 }];

/* G01/G02 —— 真实案例:北京市西城区人民政府行政复议决定(2026-01-27 05:29,
 * 外埠车无进京证驶入二环区域,违法代码 11165,罚 100 记 1 分,复议维持)
 * https://www.bjxch.gov.cn/xxgk/xxxq/pnidpv994223.html */
let g = L.analyze(39.9087, 116.3975, D('2026-01-27T05:29:00'), gbase);
eq('G01 复议案例:无证进二环区域 -> 需办证', g.verdict, 'permit');
eq('G01 复议案例:非高峰(05:29)', g.peak, null);
g = L.analyze(39.9087, 116.3975, D('2026-01-27T05:29:00'), { ...gbase, records: permitRec });
eq('G02 复议案例:持证同路段 -> 可通行', g.verdict, 'ok');

/* G03/G04 —— 市政府通告括注"工作日(因法定节假日放假调休而调整为上班的星期六、星期日除外)"
 * https://www.beijing.gov.cn/zhengce/zhengcefagui/202603/t20260323_4564097.html */
g = L.analyze(39.9087, 116.3975, D('2026-10-10T08:00:00'), { ...gbase, records: octRec });
eq('G03 补班周六 08:00 持证 -> 可通行', g.verdict, 'ok');
g = L.analyze(39.9087, 116.3975, D('2026-10-10T08:00:00'), gbase);
eq('G04 补班周六 08:00 无证 -> 需办证(非禁行)', g.verdict, 'permit');

/* G05 —— 法定节假日(国庆)工作日晨不限行 */
g = L.analyze(39.9087, 116.3975, D('2026-10-05T08:00:00'), { ...gbase, records: octRec2 });
eq('G05 国庆周一 08:00 持证 -> 可通行', g.verdict, 'ok');

/* G06/G07 —— 高峰禁行 + 处罚披露(jtgl 官方问答:罚 100 记 1 分) */
g = L.analyze(39.9087, 116.3975, D('2026-11-03T08:00:00'), { ...gbase, records: novRec });
eq('G06 工作日 08:00 持证进五环 -> 禁止', g.verdict, 'bad');
ok('G06 处罚披露含 100 元/记 1 分', g.lines.some((l) => l[0] === '处罚' && /100 元/.test(l[1]) && /记 1 分/.test(l[1])));
eq('G07 08:59 仍属早高峰', L.peakOf(D('2026-11-03T08:59:00'), 'auto').key, 'am');
eq('G07 09:00 高峰结束', L.peakOf(D('2026-11-03T09:00:00'), 'auto'), null);

/* G08/G09 —— 尾号限行(2026-09-28~12-27 轮换:周一 5/0、周二 1/6 …)
 * 市政府通告第三条 + jtgl 问答(罚 100 不记分) */
g = L.analyze(39.9087, 116.3975, D('2026-11-03T10:00:00'), { ...gbase, records: novRec, plate: '6' });
eq('G08 周二尾号 6 10:00 持证 -> 尾号禁行', g.verdict, 'bad');
eq('G08 尾号冲突标记', g.tailBlocked, true);
g = L.analyze(39.9087, 116.3975, D('2026-11-03T10:00:00'), { ...gbase, records: novRec, plate: '9' });
eq('G09 周二尾号 9 10:00 持证 -> 可通行', g.verdict, 'ok');

/* G10/G11 —— 字母尾号按 0 管理(通告第三条) */
g = L.analyze(39.9087, 116.3975, D('2026-11-02T10:00:00'), { ...gbase, records: novRec, plate: 'A' });
eq('G10 周一字母(=0)10:00 -> 尾号禁行', g.verdict, 'bad');
g = L.analyze(39.9087, 116.3975, D('2026-11-03T10:00:00'), { ...gbase, records: novRec, plate: 'A' });
eq('G11 周二字母(=0)10:00 -> 可通行', g.verdict, 'ok');

/* G12 —— 周末尾号不限行 */
g = L.analyze(39.9087, 116.3975, D('2026-11-07T10:00:00'), { ...gbase, records: novRec2, plate: '6' });
eq('G12 周六尾号 6 -> 可通行', g.verdict, 'ok');

/* G13 —— 高峰优先于尾号(08:00 属 7-9 窗口) */
g = L.analyze(39.9087, 116.3975, D('2026-11-03T08:00:00'), { ...gbase, records: novRec, plate: '9' });
eq('G13 08:00 高峰禁行(与尾号无关)', g.verdict, 'bad');

/* G14 —— 未填尾号:披露而非误判 */
g = L.analyze(39.9087, 116.3975, D('2026-11-03T10:00:00'), { ...gbase, records: novRec });
eq('G14 未填尾号 -> 不误判(持证可通行)', g.verdict, 'ok');
eq('G14 尾号未知标记', g.tail && g.tail.known, false);

/* G15 —— 尾号窗口边界 16:59 / 17:00 */
g = L.analyze(39.9087, 116.3975, D('2026-11-03T16:59:00'), { ...gbase, records: novRec, plate: '6' });
eq('G15 16:59 尾号 6 -> 禁行', g.verdict, 'bad');
g = L.analyze(39.9087, 116.3975, D('2026-11-03T17:00:00'), { ...gbase, records: novRec, plate: '6' });
eq('G15 17:00 转入晚高峰 -> 禁行', g.verdict, 'bad');

/* G16 —— 轮换表截至 2027-03-28,之后日期披露待核 */
g = L.analyze(39.9087, 116.3975, D('2027-04-01T10:00:00'),
  { ...gbase, records: [{ date: '2027-03-30', days: 7 }] });
eq('G16 轮换表外日期 -> 不误判', g.verdict, 'ok');
eq('G16 尾号组为 null(待核)', g.tail, null);

/* G17/G18 —— 区域:六环外高峰无约束 / 通州需证但不受五环高峰 */
g = L.analyze(39.5086, 116.4109, D('2026-11-03T08:00:00'), gbase);
eq('G17 大兴机场 08:00 -> 可通行', g.verdict, 'ok');
g = L.analyze(39.9060, 116.6460, D('2026-11-03T08:00:00'), gbase);
eq('G18 通州 08:00 -> 需办证(不受五环高峰)', g.verdict, 'permit');

/* G19 —— 年度次数用完 */
g = L.analyze(39.9087, 116.3975, D('2026-11-03T10:00:00'),
  { ...gbase, limit: 1, records: [{ date: '2026-01-01', days: 7 }] });
eq('G19 次数用完 -> 禁止', g.verdict, 'bad');

/* G20 —— 法源引用随结论输出 */
g = L.analyze(39.9087, 116.3975, D('2026-11-03T08:00:00'), { ...gbase, records: novRec });
ok('G20 结论引用 peak-five-ring', g.ruleIds.includes('peak-five-ring'));
ok('G20 引用含原文链接与核对日期',
  g.citations.some((c) => c.id === 'peak-five-ring' && c.url && c.verified_at === L.VERIFIED_AT));
ok('G20 需证结论引用 permit-required', g.ruleIds.includes('permit-required'));

/* G21 —— 未覆盖规则披露(二环全天禁行 / 大兴顺义高峰) */
ok('G21 五环内披露 ring2-all-day', g.gaps.some((x) => x.id === 'ring2-all-day'));
ok('G21 工作日披露 daxing-shunyi-peak', g.gaps.some((x) => x.id === 'daxing-shunyi-peak'));

/* G22 —— 车辆假设显式声明 */
ok('G22 车辆假设行存在', g.lines.some((l) => l[0] === '车辆假设' && /新能源/.test(l[1])));

/* G23 —— 无证进五环的处罚披露(京交发〔2018〕102 号 + 复议案例) */
g = L.analyze(39.9087, 116.3975, D('2026-11-03T10:00:00'), gbase);
eq('G23 无证 -> 需办证', g.verdict, 'permit');
ok('G23 处罚披露含 100 元/记 1 分', g.lines.some((l) => l[0] === '处罚' && /100 元/.test(l[1]) && /记 1 分/.test(l[1])));

/* G24 —— 贴近五环外侧(约 1.3km):边界不确定披露 */
g = L.analyze(39.7448, 116.3750, D('2026-11-03T10:00:00'), gbase);
eq('G24 五环外侧 -> 六环区', g.zone, 'six');
eq('G24 边界不确定标记', g.uncertain, true);

/* G25 —— 路线体检的尾号冲突 */
let gr = L.analyzeRoute(routeVerts, D('2026-11-03T10:00:00'), { ...rBase, records: novRec, plate: '6' });
eq('G25 路线 10:00 进五环 尾号 6 -> 禁止', gr.verdict, 'bad');
eq('G25 尾号冲突识别', gr.tailConflict, true);
gr = L.analyzeRoute(routeVerts, D('2026-11-03T10:00:00'), { ...rBase, records: novRec, plate: '9' });
eq('G25b 路线 尾号 9 -> 合规', gr.verdict, 'ok');

console.log('\n结果: ' + pass + ' passed, ' + fail + ' failed');
if (fail) process.exit(1);
