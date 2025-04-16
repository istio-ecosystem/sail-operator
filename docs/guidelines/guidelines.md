[Return to Documentation Dir](../docs/README.md) # Path: docs/README.md

# Documentation Guidelines

## Introduction
This guide aims to set basic guidelines for writing documentation for the project. The documentation should be clear, concise, and easy to understand. It should be written in a way that is accessible to both technical and non-technical users.

## Documentation Structure
The documentation should be structured in a way that is easy to navigate. It should be divided into sections and subsections, with a table of contents at the beginning of the document. Each section should cover a specific topic, and each subsection should cover a specific aspect of that topic.

### Structure of the entire folder and new folders
All the documentation lives unders the folder `docs` in the root of the project. The documentation is divided into the following sections:
- `guidelines`: Contains guidelines for writing documentation.
- `api-reference`: Contains the API reference documentation. It is generated from the code, so if you want to update it, you need to update the docstrings.
- `common`: Contains common documentation that is relevant to the entire project, for example: `create-and-configure-gateways`, `install-bookinfo-app`, etc. All the docs located here should be linked to from the main README.md file for easy access.
- `multicluster`: contains resources referenced in the README.md file for the multicluster setup.
- `README.md`: The main README.md file that contains the project main doc and links to the other documentation topics.

Any new folder or file added to the documentation should be linked to from the main README.md and you should only create a new folder if the documentation is not relevant to the entire project or if it is a new section that does not fit into the existing structure. Also, new folders can contain resources that are referenced in the README.md to be able to run examples or to get more information about the project (for an example, see the `multicluster` folder).

### Table of Contents
For long documents, it is recommended to include a table of contents at the beginning of the document. The table of contents should list all the sections and subsections in the document.

### Headings
Use headings to break up the content into sections and subsections. Headings should be descriptive and should clearly indicate the topic of the section or subsection.

### Formatting
Use formatting to make the text more readable. Use bullet points for lists, code blocks for code snippets, and inline code for short code snippets. Use bold or italic text to highlight important points. Any topic that is important to the reader should be highlighted in some way.

### Links
Use links to reference other sections of the documentation, external resources, or related topics. Links should be descriptive and should clearly indicate the target of the link. Links should be used to provide additional information or context for the reader. Avoid the use of raw URLs in the documentation.

### Images
Use images to illustrate concepts and provide visual guidance. Images should be accompanied by explanations and annotations to help the reader understand the content. Images should be used to enhance the text and provide additional context for the reader and they are going to be stored in a `images` folder.

### Nomemclature
Use consistent terminology throughout the documentation. Use the same terms to refer to the same concepts and avoid using different terms to refer to the same concept. For this project, we are going to use the following terms:
- `Sail Operator` to refer to the Sail Operator project.
- `istio` to refer to the Istio service mesh upstream project.
- `Istio` to the resource created and managed by the Sail Operator.
- `IstioCNI` to refer to the Istio CNI resource managed by the Sail Operator.
- `istio-cni` to refer to the Istio CNI project.

### Examples
Use examples to illustrate concepts and provide practical guidance. Examples should be clear, concise, and easy to follow. They should be accompanied by explanations and annotations to help the reader understand the code. Also, the examples provided can be used to run automated tests over the example steps. This is going to be explained in the next section.

Use code block for command or groups of command that has the same context. Use validation steps to ensure that conditions are met and avoid flakiness of the test, in the next section we are going to explain how to add validation steps to the examples. Also, use the `ignore` tag to ignore code blocks that are not going to be run by the automation tool. For example, yaml files or any other code block that is not going to be run in the terminal.

## Automated Testing over the Documentation
Any documentation step need to be tested to ensure that the steps are correct and the user can follow them without any issue. To do this, we use `runme` (check the docs [here](https://docs.runme.dev/getting-started/cli)) to run the tests over the documentation. The workflow of the automation is the following:

- The documentation files are in the `docs` folder.
- The `make update` target is going to run the script `tests/documentation_test/scripts/update-docs-examples.sh` that is going to generate the md modified files to run the test (For more information check next topic).
- The `make test.docs` target is going to run the script `tests/documentation_test/scripts/run-docs-examples.sh` that is going to run the tests over the documentation files. This script is going to run all the steps in the examples marked following the guidelines explained in the next section.

### What does update-docs-examples.sh do?
The script `update-docs-examples.sh` is going to run the following steps:
- Check all the md files in the `docs` folder and exclude from the list the files described inside the `EXCLUDE_FILES` variable. The resulting list will be all the .md files that contains at least one time this pattern `bash { name=`. This pattern is used by the `runme` tool to generate the resulting jupyter notebook by looking into all the code blocks that has certain field decorators. The field decorators and how to use them are explained in the next section.
- Once the files are copied into the destination path we check those files to uncomment the bash commented steps. This bash commented code block are going to be hiden in the original md files but we will uncomment this code block to be able to run validation steps that wait for conditions an avoid flakiness of the test. This means that most of the validation steps will be commented code blocks.
``

### Adding a topic example to the automation workflow
To add a new topic to the automation workflow you need to:
1. In your documentation topic, each bash code block that you want to execute as part of the automation must include the following pattern:
- `bash { name=example-name tag=example-tag }`: the fields used here are:
  - `name`: the name of the example step, this is usefull to identify the step in the output of the test. The name should be short and descriptive. For example: `deploy-operator`, `wait-operator`, etc.
  - `tag`: the tag of the example that is going to be used to run the test, is important to be a unique name. This tag should be unique and should not be used in other examples. The tag can be used to run only a specific example inside a file.
For example:

````md
```bash { name=deploy-operator tag=example}
helm repo add sail-operator https://istio-ecosystem.github.io/sail-operator
```
````

- `bash { ignore=true }`: this pattern is used to ignore the code block when they are going to be run by the `runme` tool. This is used to ignore or avoid the excution of yaml examples (because `runme` threat everything as a bash command) or any other code block that you do not want to run. This is going to be used in the examples that are going to be run by the `runme` tool. For example

````md
- Example of a Istio resource:
```yaml { ignore=true }
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: RevisionBased
    inactiveRevisionDeletionGracePeriodSeconds: 30
  version: v1.24.2
```
````

*Note:* take into account that all the code blocks that are going to be run by the `runme` tool should be tagged with the language `bash` at the beginning of the block and they need to contain only commands that are going to be run in the terminal and all the steps that are not needed to be executed because they are not commands or they are not needed for the goal of the example should be ignored with the `ignore=true` tag. For example

````md
- You shouuld set version for Istio in the `Istio` resource and `IstioRevisionTag` resource should reference the name of the `Istio` resource:
```yaml { ignore=true }
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: RevisionBased
    inactiveRevisionDeletionGracePeriodSeconds: 30
  version: v1.24.2
---
apiVersion: sailoperator.io/v1
kind: IstioRevisionTag
metadata:
  name: default
spec:
  targetRef:
    kind: Istio
    name: default
```
- Create ns, `Istio` and `IstioRevisionTag` resources:
```bash { name=create-istio tag=example}
kubectl create ns istio-system
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: RevisionBased
    inactiveRevisionDeletionGracePeriodSeconds: 30
  version: v1.24.2
---
apiVersion: sailoperator.io/v1
kind: IstioRevisionTag
metadata:
  name: default
spec:
  targetRef:
    kind: Istio
    name: default
EOF
```
````

- Validation steps are always needed, to add a validation step you can comment directly the entire code block with the bash command that you want to run. This validation steps need to use the same tag as the example where they are going to run and should be prefixed with `validation-description`. For example:
```md
<!-- ```bash { name=validation-wait-operator tag=example}
kubectl wait --for=condition=available --timeout=600s deployment/sail-operator -n sail-operator
``` -->
```

*Important:* please always add validation steps to avoid flakyness during the execution, these steps will ensure that resource conditions are met and the test will not fail because of a timeout or any other issue. This validation needs to fail the test if the condition is not met, by returning a non-zero return code. Use `kubectl wait` over `sleep` wherever possible.

*Note:* if you want to check all the commands that inside a md file you can execute the following command:
```bash
runme list --filename docs/common/runme-test.md
```
This will output the entire list of commands but does not have filter for the tags.