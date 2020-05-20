// eslint-disable-next-line @typescript-eslint/no-var-requires
const path = require('path')

const extensions = ['.tsx', '.ts', '.js', 'json']

module.exports = {
  entry: './src/index.ts',
  devtool: 'inline-source-map',
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: 'ts-loader',
        exclude: /node_modules/,
      },
    ],
  },
  resolveLoader: {
    modules: ['../../node_modules'],
  },
  resolve: {
    modules: ['./node_modules'],
    extensions,
    symlinks: false,
    alias: {
      '@textile/textile': path.resolve(__dirname, 'packages/textile'),
    },
  },
  output: {
    filename: './[name].js',
    path: path.resolve(process.cwd(), 'dist'),
    library: 'textile',
    libraryTarget: 'var',
  },
  optimization: {
    splitChunks: {
      chunks: 'all',
    },
    minimize: true,
  },
}
