import ts from 'typescript';

const compilerOptions = {
  target: ts.ScriptTarget.ES2022,
  module: ts.ModuleKind.ES2022,
  moduleResolution: ts.ModuleResolutionKind.Bundler,
  esModuleInterop: true,
  sourceMap: 'inline',
};

export async function load(url, context, defaultLoad) {
  if (!url.endsWith('.ts')) {
    return defaultLoad(url, context, defaultLoad);
  }

  const result = await defaultLoad(url, { ...context, format: 'module' }, defaultLoad);
  const source = Buffer.isBuffer(result.source)
    ? result.source.toString('utf8')
    : String(result.source);

  const transpiled = ts.transpileModule(source, {
    compilerOptions,
    fileName: new URL(url).pathname,
  });

  return {
    format: 'module',
    shortCircuit: true,
    source: transpiled.outputText,
  };
}
