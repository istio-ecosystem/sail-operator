[Return to Project Root](../../README.md)

# Table of Contents

- [Documentation Guidelines](#documentation-guidelines)
  - [Introduction](#introduction)
  - [Documentation Structure](#documentation-structure)
    - [Structure of the entire folder and new folders](#structure-of-the-entire-folder-and-new-folders)
    - [Table of Contents](#table-of-contents)
    - [Headings](#headings)
    - [Formatting](#formatting)
    - [Links](#links)
    - [Images](#images)
    - [Nomenclature](#nomenclature)
    - [Examples](#examples)
  - [Automated Testing over the Documentation](#automated-testing-over-the-documentation)
    - [What does update-docs-examples.sh do?](#what-does-update-docs-examplessh-do)
    - [Adding a topic example to the automation workflow](#adding-a-topic-example-to-the-automation-workflow)
    - [Debugging a specific docs example locally](#debugging-a-specific-docs-example-locally)

# Documentation Guidelines

## Introduction
This guide aims to set basic guidelines for writing documentation for the project. The documentation should be clear, concise, and easy to understand. It should be written in a way that is accessible to both technical and non-technical users.

## Documentation Structure
The documentation should be structured in a way that is easy to navigate. It should be divided into sections and subsections, with a table of contents at the beginning of the document. Each section should cover a specific topic, and each subsection should cover a specific aspect of that topic.

### Structure of the entire folder and new folders
All the documentation lives under the folder `docs` in the root of the project. The documentation is divided into the following sections:
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

### Nomenclature
Use consistent terminology throughout the documentation. Use the same terms to refer to the same concepts and avoid using different terms to refer to the same concept. For this project, we are going to use the following terms:
- `Sail Operator` to refer to the Sail Operator project.
- `istio` to refer to the Istio service mesh upstream project.
- `Istio` to the resource created and managed by the Sail Operator.
- `IstioCNI` to refer to the Istio CNI resource managed by the Sail Operator.
- `istio-cni` to refer to the Istio CNI project.
- `ztunnel` to refer to the Istio ztunnel upstream project.
- `ZTunnel` to the resource created and managed by the Sail Operator.

### Examples
Use examples to illustrate concepts and provide practical guidance. Examples should be clear, concise, and easy to follow. They should be accompanied by explanations and annotations to help the reader understand the code. Also, the examples provided can be used to run automated tests over the example steps. This is going to be explained in the next section.

Use code block for command or groups of command that has the same context. Use validation steps to ensure that conditions are met and avoid flakiness of the test, in the next section we are going to explain how to add validation steps to the examples. Also, use the `ignore` tag to ignore code blocks that are not going to be run by the automation tool. For example, yaml files or any other code block that is not going to be run in the terminal.

## Automated Testing over the Documentation
Any documentation step need to be tested to ensure that the steps are correct and the user can follow them without any issue. To do this, we use `runme` (check the docs [here](https://docs.runme.dev/getting-started/cli)) to run the tests over the documentation. The workflow of the automation is the following:

- The documentation files are in the `docs` folder.
- The `make test.docs` target temporarily copies the documentation files to the `ARTIFACTS` directory (defaulting to a temporary folder) and uncomments all commented validation steps. This process is managed by the `tests/documentation_test/scripts/update-docs-examples.sh` script. Once the files are prepared, the `tests/documentation_test/scripts/run-docs-examples.sh` script executes the tests on the documentation files. It runs all example steps marked according to the guidelines described in the following sections. After the tests are completed, the temporary files are cleaned up.

### What does update-docs-examples.sh do
The script `update-docs-examples.sh` is going to run the following steps:
- Check all the md files in the `docs` folder and exclude from the list the files described inside the `EXCLUDE_FILES` variable and then copy all the files to the `tests/documentation_test/` folder that meets the criteria, check [Adding a topic example to the automation workflow](#automated-testing-over-the-documentation) section for more information about the criteria.
- Once the files are copied into the destination path it checks those files to uncomment the bash commented steps. This bash commented code block are going to be hiden in the original md files but we will uncomment this code block to be able to run validation steps that wait for conditions an avoid flakiness of the test. This means that most of the validation steps will be commented code blocks, more information [here](#adding-a-topic-example-to-the-automation-workflow).


### Adding a topic example to the automation workflow
To add a new topic to the automation workflow you need to:
1. In your documentation topic, each bash code block that you want to execute as part of the automation must include the following pattern:
- `bash { name=example-name tag=example-tag }`: the fields used here are:
  - `name`: the name of the example step, this is useful to identify the step in the output of the test. The name should be short and descriptive. For example: `deploy-operator`, `wait-operator`, etc.
  - `tag`: the tag of the example that is going to be used to run the test, is important to be a unique name. This tag should be unique and should not be used in other examples. The tag can be used to run only a specific example inside a file.
For example:

````md
```bash { name=deploy-operator tag=example-tag}
helm repo add sail-operator https://istio-ecosystem.github.io/sail-operator
```
````

- use `bash { ignore=true }` to prevent a code block from being executed by the `runme` tool. This is helpful when you include YAML examples or other content that shouldn’t be part of automation workflow. Using `ignore` attribute ensures only the intended code blocks are run. For example:

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

> [!Note]  
> When using the `runme` tool, make sure that all code blocks intended for execution are marked with the bash language at the start of the block. These blocks should include only commands that are meant to be run in the terminal. Any steps that are not actual commands or are not essential for the goal of the example should be skipped using the `ignore=true` tag. For example:

````md
- You should set version for Istio in the `Istio` resource and `IstioRevisionTag` resource should reference the name of the `Istio` resource:
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
```bash { name=create-istio tag=example-tag}
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
````md
<!--
```bash { name=validation-wait-operator tag=example-tag}
kubectl wait --for=condition=available --timeout=600s deployment/sail-operator -n sail-operator
```
-->
````

To avoid putting duplicated validation steps and help the users to easily get information, validate steps, etc. you can use prebuilt validation steps that are already created in the [prebuilts-func.sh](../../tests/documentation_tests/scripts/prebuilt-func.sh) script. For example, if you want to check if the istiod pod is ready you can use the `wait_istio_ready` function that is already created in the script. To use this function you need to add the following code block in your documentation:
````md
<!--
```bash { name=validation-wait-operator tag=example-tag}
. $SCRIPT_DIR/prebuilt-func.sh
wait_istio_ready "istio-system"
```
-->
````
To check the entire list of prebuilt functions please check the [prebuilts-func.sh](../../tests/documentation_tests/scripts/prebuilt-func.sh) script. Note that `SCRIPT_DIR` is a variable that is already defined in the `run-docs-examples.sh` script, so you can use it directly in your documentation.

> [!IMPORTANT]  
> Always include validation steps to avoid flakiness. They ensure resources are in expected conditions and the test fails clearly if they don’t.

> [!Note] 
> If you want to check all the commands that inside a md file you can execute the following command:
```bash
runme list --filename docs/common/runme-test.md
```
This will output the entire list of commands but does not have filter for the tags.

### Debugging a specific docs example locally
To debug a specific example locally, you can use the the make target with the use of the env variable `FOCUS_DOC_TAGS`. For example, if you want to run the example with the tag `example-tag` you can run the following command:

```bash
make test.docs FOCUS_DOC_TAGS=example-tag
```

This will run only the example with the tag `example-tag` and will not run any other example. This is useful to debug a specific example without running all the examples in the documentation. Take into account that the make target already creates their own kind cluster and installs the Sail Operator, so you don't need to do it manually. Also, the make target will clean up the cluster after the test is finished.
