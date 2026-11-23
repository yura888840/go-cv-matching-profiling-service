#!/usr/bin/env python3
"""Linux reproducible CPU-time / peak-RSS comparison, no Python dependencies.

Run from the repository root: python3 scripts/compare.py --cvs 500000 --iterations 200
The measurements include the generated CV data footprint, but do not include Go compilation.
"""
import argparse
import datetime
import json
import os
from pathlib import Path
import platform
import resource
import subprocess
import sys
import tempfile
import time


def run_mode(binary, mode, cvs, iterations):
    args = [str(binary), '-mode', mode, '-workers', '4', '-cvs', str(cvs), '-iterations', str(iterations)]
    start = time.perf_counter()
    pid = os.posix_spawn(str(binary), args, os.environ.copy(), file_actions=[(os.POSIX_SPAWN_OPEN, 1, os.devnull, os.O_WRONLY, 0o644)])
    _, status, usage = os.wait4(pid, 0)
    elapsed = time.perf_counter() - start
    if not os.WIFEXITED(status) or os.WEXITSTATUS(status) != 0:
        raise RuntimeError(f'{mode} failed: {status}')
    return {'mode': mode, 'wall_seconds': round(elapsed, 6), 'cpu_user_seconds': round(usage.ru_utime, 6),
            'cpu_system_seconds': round(usage.ru_stime, 6),
            'cpu_average_percent_of_one_core': round(100 * (usage.ru_utime + usage.ru_stime) / elapsed, 2),
            'peak_rss_mib': round(usage.ru_maxrss / 1024, 2),
            'cv_per_second': round(cvs * iterations / elapsed)}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--cvs', type=int, default=500_000)
    p.add_argument('--iterations', type=int, default=200)
    p.add_argument('--samples', type=int, default=3)
    p.add_argument('--out', default='docs/benchmarks/results.json')
    args = p.parse_args()
    if sys.platform != 'linux':
        p.error('This command uses Linux ru_maxrss units; run on Linux or adapt RSS conversion')
    if args.cvs < 1 or args.iterations < 1 or args.samples < 1:
        p.error('all numeric values must be positive')
    with tempfile.TemporaryDirectory() as tmp:
        binary = Path(tmp) / 'profilematch'
        subprocess.run(['go', 'build', '-o', str(binary), './cmd/profilematch'], check=True)
        # Exclude cold-start effects from samples.
        for mode in ('sequential', 'worker_pool'):
            run_mode(binary, mode, max(1000, min(args.cvs, 10000)), 2)
        samples = []
        for index in range(args.samples):
            for mode in (('sequential', 'worker_pool') if index % 2 == 0 else ('worker_pool', 'sequential')):
                samples.append(run_mode(binary, mode, args.cvs, args.iterations))
    payload = {'date_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
               'platform': platform.platform(), 'python': platform.python_version(),
               'go_version': subprocess.check_output(['go', 'version'], text=True).strip(),
               'cvs': args.cvs, 'iterations': args.iterations, 'samples_per_mode': args.samples,
               'scope': 'Standalone matching CLI; process wall/CPU and peak RSS; synthetic CVs, no HTTP/DB',
               'results': samples}
    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(payload, indent=2) + '\n')
    print(json.dumps(payload, indent=2))


if __name__ == '__main__':
    main()
