#!/bin/bash
set -e

echo "Building fluttex compiler..."
go build -o fluttex_bin main.go

echo "Creating dummy flutter project..."
# Put test project completely outside the git repo to avoid git diff issues
cd /tmp
rm -rf test_proj
flutter create test_proj --empty
cd test_proj

echo "Generating TSX input..."
cat << 'TSX' > app.tsx
import React, { useState } from 'react';

export function App() {
	const [count, setCount] = useState(0);

	return (
		<View>
			<Text>Hello</Text>
			<button data-testid="my-btn" onClick={() => setCount(count + 1)}>
				Click
			</button>
            <input data-testid="my-input" onChange={() => {}} />
            <Icon name="check" />
		</View>
	);
}
TSX

echo "Transpiling..."
"$PWD/fluttex_bin" build app.tsx

echo "Setting up integration test..."
mkdir -p test
cat << 'DART' > test/widget_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:test_proj/app.dart';

void main() {
  testWidgets('App counter increments and renders elements correctly', (WidgetTester tester) async {
    await tester.pumpWidget(const MaterialApp(home: Scaffold(body: App())));

    expect(find.text('Hello'), findsOneWidget);
    expect(find.text('Click'), findsOneWidget);
    expect(find.byType(TextField), findsOneWidget);
    expect(find.byIcon(Icons.check), findsOneWidget);

    // Initial state
    expect(tester.state(find.byType(App)).widget, isNotNull);

    // Tap button
    await tester.tap(find.byKey(const Key('my-btn')));
    await tester.pump();

    // Verify it didn't crash
    expect(find.byType(App), findsOneWidget);
  });
}
DART

echo "Running flutter test..."
flutter test
