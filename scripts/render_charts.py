#!/usr/bin/env python3
"""Render standalone SVG charts using only the Python standard library."""
from pathlib import Path
import json
import statistics

root = Path('docs/benchmarks')
results = json.loads((root / 'results.json').read_text())
colors = {'sequential': '#3b82f6', 'worker_pool': '#f59e0b'}
names = {'sequential': 'Sequential', 'worker_pool': '4 workers'}


def median(mode, key):
    values = [s[key] for s in results['results'] if s['mode'] == mode]
    return statistics.median(values)


def render(filename, title, key, unit, note, maximum=None):
    values = {mode: median(mode, key) for mode in names}
    bound = maximum or max(values.values()) * 1.2
    out = [
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 286" role="img" aria-label="' + title + '">',
        '<rect width="800" height="286" rx="14" fill="#0b1220"/>',
        '<text x="30" y="43" fill="#f8fafc" font-size="23" font-family="Arial,sans-serif" font-weight="700">' + title + '</text>',
    ]
    for i, mode in enumerate(names):
        y = 84 + i * 79
        out += [
            f'<text x="30" y="{y}" fill="#cbd5e1" font-size="17" font-family="Arial,sans-serif">{names[mode]}</text>',
            f'<rect x="210" y="{y-21}" width="490" height="29" rx="5" fill="#243244"/>',
            f'<rect x="210" y="{y-21}" width="{490*values[mode]/bound:.2f}" height="29" rx="5" fill="{colors[mode]}"/>',
            f'<text x="710" y="{y}" fill="#f8fafc" font-size="19" font-family="Arial,sans-serif">{values[mode]:,.2f} {unit}</text>',
        ]
    out += [f'<text x="30" y="258" fill="#94a3b8" font-size="13" font-family="Arial,sans-serif">{note}</text>', '</svg>']
    (root / filename).write_text('\n'.join(out) + '\n')


if __name__ == '__main__':
    render('wall-time.svg', 'Total matching time (lower is better)', 'wall_seconds', 's', '500k CVs x 200 iterations · 3 samples per mode · median')
    render('cpu.svg', 'Average CPU usage (one core = 100%)', 'cpu_average_percent_of_one_core', '%', 'CPU average, NOT instantaneous peak · Linux process accounting', 440)
    render('memory.svg', 'Peak resident memory (RSS)', 'peak_rss_mib', 'MiB', 'Peak RSS including generated CVs · Linux ru_maxrss', 100)
