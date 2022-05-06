@Library('jenkins-library@master') _
dockerImagePipeline1(
  script: this,
  service: 'aveshadev/worker-operator',
  dockerfile: 'Dockerfile',
  buildContext: '.',
  buildArguments: [PLATFORM:"amd64"]
)
