---
title: "JUnit lens"
weight: 20
description: >
  
---

Parses and displays JUnit XML test results summarized by result, with optional filtering to group testcases by properties.

## Configuration

The JUnit lens supports optional configuration to group testcases based on their XML properties:

* `groups` - (optional) List of test groups to segregate tests based on properties. Each group entry has:
  * `name` - Display name for the group section header
  * `selector` - XPath property selector expression (currently supports `properties/property[@name='X' and @value='Y']` syntax)
  * `collapsed` - Boolean, whether the group section starts collapsed (default: `false`)
  * `include_suite_properties` - Boolean, whether the selector also uses properties inherited from enclosing `<testsuite>` elements (default: `false`). Testcase properties override suite properties with the same name; nearer suites override ancestor suites. This option applies only to this group.

Tests matching a group selector are displayed in a separate group section with its name as header and the group's pass/fail/skip/flaky breakdown. Tests that don't match any group remain in a default group at the beginning.

## Example Configuration

```yaml
deck:
  spyglass:
    lenses:
    - lens:
        name: junit
        config:
          groups:
          - name: "Informing Tests"
            selector: "properties/property[@name='lifecycle' and @value='informing']"
            collapsed: true
            include_suite_properties: true
          - name: "Upgrade Tests"
            selector: "properties/property[@name='type' and @value='upgrade']"
            collapsed: false
      required_files:
      - ^artifacts/junit.*\.xml$
```

## Expected Input

JUnit XML files with optional `<properties>` elements within `<testcase>` elements:

```xml
<testsuites>
  <testsuite>
    <testcase classname="MyClass" name="test_something">
      <properties>
        <property name="lifecycle" value="informing"/>
      </properties>
    </testcase>
  </testsuite>
</testsuites>
```

Groups with `include_suite_properties: true` also match suite-level properties. For
example, the "Informing Tests" group above matches `test_inherited` below, while
`test_override` remains in the default group because its testcase property overrides
the suite's `lifecycle` value:

```xml
<testsuites>
  <testsuite name="parent">
    <properties>
      <property name="lifecycle" value="informing"/>
    </properties>
    <testsuite name="nested">
      <testcase name="test_inherited"/>
      <testcase name="test_override">
        <properties>
          <property name="lifecycle" value="blocking"/>
        </properties>
      </testcase>
    </testsuite>
  </testsuite>
</testsuites>
```

Groups without this option continue to inspect testcase properties only. Suite
properties do not carry over to sibling suites. Group order still determines the
first match, including when groups use different settings for this option.

## Selector Syntax

Currently, only property predicates are supported. The selector syntax is:

```
properties/property[@name='<property-name>' and @value='<property-value>']
```

Both `@name` and `@value` are optional:
- `properties/property[@name='lifecycle']` - matches any property with name "lifecycle" regardless of value
- `properties/property[@value='informing']` - matches any property with value "informing" regardless of name
- `properties/property[@name='lifecycle' and @value='informing']` - matches properties with both name and value

## Behavior

- Group selectors are processed in order; a test matches the first group whose selector matches, if any
- Tests that don't match any group appear in the default group at the top
- Each group section shows its own summary (X/Y Failed, Passed, Skipped, Flaky)
- The `collapsed` setting controls whether the group's test details are initially hidden
