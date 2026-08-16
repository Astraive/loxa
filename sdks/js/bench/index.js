const result = Bun.spawnSync(['bunx', 'vitest', 'bench', '--run'], {
  stdout: 'pipe',
  stderr: 'pipe',
});

process.stdout.write(result.stdout);
process.stderr.write(result.stderr);
process.exit(result.exitCode ?? 1);
