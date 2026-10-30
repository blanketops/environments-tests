#!/usr/bin/env python3
"""
Convert Ginkgo JSON report to JUnit XML with individual test cases per spec.
This allows Allure to show each Ginkgo spec as a separate test case.
"""

import json
import sys
import os
from xml.etree.ElementTree import Element, SubElement, tostring
from xml.dom import minidom
from datetime import datetime


def format_duration(runtime_ns):
    """Convert nanoseconds to seconds with 3 decimal places."""
    return f"{runtime_ns / 1e9:.3f}"


def build_junit_from_ginkgo(ginkgo_report):
    """Build JUnit XML from Ginkgo JSON report."""
    testsuites = Element('testsuites')
    
    for suite in ginkgo_report:
        suite_name = suite.get('SuiteDescription', 'Ginkgo Suite')
        suite_path = suite.get('SuitePath', '')
        
        testsuite = SubElement(testsuites, 'testsuite', {
            'name': suite_name,
            'tests': str(suite['PreRunStats']['TotalSpecs']),
            'failures': str(len([s for s in suite['SpecReports'] if s['State'] == 'failed'])),
            'errors': '0',
            'skipped': str(len([s for s in suite['SpecReports'] if s['State'] == 'skipped'])),
            'time': f"{suite['RunTime'] / 1e9:.3f}",
            'timestamp': suite['StartTime'].replace('Z', '+00:00'),
        })
        
        # Add properties
        properties = SubElement(testsuite, 'properties')
        SubElement(properties, 'property', {'name': 'suite_path', 'value': suite_path})
        SubElement(properties, 'property', {'name': 'random_seed', 'value': str(suite['SuiteConfig']['RandomSeed'])})
        
        for spec in suite['SpecReports']:
            # Only process actual test specs (It, Specify)
            if spec['LeafNodeType'] not in ('It', 'Specify'):
                continue
            
            test_name = spec['LeafNodeText']
            if not test_name:
                continue
            
            # Build full test name with container hierarchy
            container_texts = spec.get('ContainerHierarchyTexts', [])
            if container_texts:
                full_name = ' '.join(container_texts) + ' ' + test_name
            else:
                full_name = test_name
            
            classname = os.path.basename(spec['LeafNodeLocation']['FileName']).replace('_test.go', '')
            
            testcase = SubElement(testsuite, 'testcase', {
                'name': full_name,
                'classname': classname,
                'time': format_duration(spec['RunTime']),
            })
            
            state = spec['State']
            if state == 'failed':
                failure = spec.get('Failure', {})
                failure_elem = SubElement(testcase, 'failure', {
                    'message': failure.get('Message', 'Test failed')[:1000],
                    'type': 'AssertionError',
                })
                # Full stack trace
                full_stack = failure.get('FullStackTrace', '')
                location = failure.get('Location', {})
                file_name = location.get('FileName', '')
                line = location.get('LineNumber', 0)
                
                cdata_content = f"File: {file_name}:{line}\n\n{full_stack}"
                if failure.get('Message'):
                    cdata_content = f"{failure['Message']}\n\n{cdata_content}"
                failure_elem.text = f"<![CDATA[{cdata_content}]]>"
            
            elif state == 'skipped':
                SubElement(testcase, 'skipped', {'message': 'Test skipped'})
    
    return testsuites


def main():
    if len(sys.argv) < 2:
        print("Usage: python3 ginkgo2junit.py <reports-dir>")
        sys.exit(1)
    
    reports_dir = sys.argv[1]
    
    for report_file in os.listdir(reports_dir):
        if report_file.endswith('-ginkgo-report.json') or report_file == 'ginkgo-report.json':
            with open(os.path.join(reports_dir, report_file), 'r') as f:
                ginkgo_report = json.load(f)
            
            testsuites = build_junit_from_ginkgo(ginkgo_report)
            
            # Pretty print XML
            rough_string = tostring(testsuites, 'utf-8')
            reparsed = minidom.parseString(rough_string)
            pretty_xml = reparsed.toprettyxml(indent="  ")
            
            out_file = os.path.join(reports_dir, report_file.replace('.json', '-junit.xml'))
            with open(out_file, 'w') as f:
                f.write(pretty_xml)
            
            print(f"Generated {out_file}")


if __name__ == '__main__':
    main()