@Library('jenkins-library@opensource-helm-pipeline') _
dockerImagePipeline1(
  script: this,
  service: 'aveshadev/worker-operator',
  dockerfile: 'Dockerfile',
  buildContext: '.',
  buildArguments: [PLATFORM:"amd64"],
  trigger_remote: 'no'
)
