// eslint-disable-next-line @typescript-eslint/no-var-requires
const path = require('path')

module.exports = {
  target: 'web',
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
    extensions: ['.tsx', '.ts', '.js'],
  },
}
