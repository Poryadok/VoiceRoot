import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/moderation_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/trust_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/report/report_sheet.dart';

const _openReportKey = Key('open_report');
const _activePageKey = Key('active_report_page');
const _activePageFieldKey = Key('active_report_page_field');

late ThemeData _appTheme;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    final noto = FontLoader(VoiceTheme.fontFamily)
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
    await noto.load();
    var flutterRoot = File(Platform.resolvedExecutable).parent;
    for (var i = 0; i < 5; i++) {
      flutterRoot = flutterRoot.parent;
    }
    final materialIconsPath = [
      flutterRoot.path,
      'bin',
      'cache',
      'artifacts',
      'material_fonts',
      'MaterialIcons-Regular.otf',
    ].join(Platform.pathSeparator);
    final materialIconsBytes = await File(materialIconsPath).readAsBytes();
    final icons = FontLoader('MaterialIcons')
      ..addFont(Future.value(ByteData.sublistView(materialIconsBytes)));
    await icons.load();

    final catalog = await VoiceTokenCatalog.load();
    _appTheme = await VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.light,
      profileAccent: catalog.profileAccentDefaults.first,
    );
  });

  group('reportFormValidationError', () {
    test('requires comment for other category', () {
      expect(
        reportFormValidationError(
          category: ReportCategories.other,
          comment: '',
          otherCategoryCommentRequired: 'required',
        ),
        'required',
      );
    });

    test('allows other category with comment', () {
      expect(
        reportFormValidationError(
          category: ReportCategories.other,
          comment: 'details',
          otherCategoryCommentRequired: 'required',
        ),
        isNull,
      );
    });

    test('allows non-other categories without comment', () {
      expect(
        reportFormValidationError(
          category: ReportCategories.spam,
          comment: '',
          otherCategoryCommentRequired: 'required',
        ),
        isNull,
      );
    });
  });

  testWidgets(
    'H report success closes the sheet and shows a dismissible notice',
    (tester) async {
      await _bindViewport(tester, const Size(1280, 800));
      final client = _FakeModerationClient(
        result: const ModerationApiOk(
          ReportSubmission(reportId: 'report-fixture'),
        ),
      );
      final focusNode = await _pumpReportPage(tester, client);
      addTearDown(focusNode.dispose);

      await _openAndSelectSpam(tester, focusNode);
      await tester.tap(find.byKey(ReportSheet.submitButtonKey));
      await tester.pumpAndSettle();

      _expectSuccessfulSubmission(tester, client, focusNode);
      await expectLater(
        find.byType(Scaffold).first,
        matchesGoldenFile('goldens/report_success_h.png'),
      );

      final close = find.descendant(
        of: find.byType(SnackBar),
        matching: find.byType(IconButton),
      );
      expect(close, findsOneWidget);
      await tester.tap(close);
      await tester.pumpAndSettle();
      expect(find.byType(SnackBar), findsNothing);
      expect(find.byKey(_activePageKey), findsOneWidget);
      expect(focusNode.hasFocus, isTrue);
    },
  );

  testWidgets('V report success keeps the page active beneath the notice', (
    tester,
  ) async {
    await _bindViewport(tester, const Size(390, 844));
    final client = _FakeModerationClient(
      result: const ModerationApiOk(
        ReportSubmission(reportId: 'report-fixture'),
      ),
    );
    final focusNode = await _pumpReportPage(tester, client);
    addTearDown(focusNode.dispose);

    await _openAndSelectSpam(tester, focusNode);
    await tester.tap(find.byKey(ReportSheet.submitButtonKey));
    await tester.pumpAndSettle();

    _expectSuccessfulSubmission(tester, client, focusNode);
    await expectLater(
      find.byType(Scaffold).first,
      matchesGoldenFile('goldens/report_success_v.png'),
    );
  });

  testWidgets('busy submit stays single and failure keeps the editable form', (
    tester,
  ) async {
    await _bindViewport(tester, const Size(390, 844));
    final pending = Completer<ModerationApiResult<ReportSubmission>>();
    final client = _FakeModerationClient(pending: pending);
    final focusNode = await _pumpReportPage(tester, client);
    addTearDown(focusNode.dispose);

    await _openAndSelectSpam(tester, focusNode);
    await tester.tap(find.byKey(ReportSheet.submitButtonKey));
    await tester.pump();
    await tester.tap(find.byKey(ReportSheet.submitButtonKey));
    await tester.pump();
    expect(client.calls, hasLength(1));

    pending.complete(
      const ModerationApiFailure(
        message: 'Report service unavailable',
        statusCode: 503,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ReportSheet.sheetKey), findsOneWidget);
    expect(find.text('Report service unavailable'), findsOneWidget);
    expect(find.byType(SnackBar), findsNothing);
    expect(client.calls, hasLength(1));
    expect(find.byKey(ReportSheet.submitButtonKey), findsOneWidget);
  });

  testWidgets('other-category validation blocks API submission', (
    tester,
  ) async {
    await _bindViewport(tester, const Size(390, 844));
    final client = _FakeModerationClient(
      result: const ModerationApiOk(
        ReportSubmission(reportId: 'unused-fixture'),
      ),
    );
    final focusNode = await _pumpReportPage(tester, client);
    addTearDown(focusNode.dispose);

    await _openReport(tester, focusNode);
    final l10n = AppLocalizations.of(
      tester.element(find.byKey(_openReportKey)),
    )!;
    await tester.tap(
      find.widgetWithText(RadioListTile<String>, l10n.reportCategoryOther),
    );
    await tester.pump();
    await tester.ensureVisible(find.byKey(ReportSheet.submitButtonKey));
    await tester.tap(find.byKey(ReportSheet.submitButtonKey));
    await tester.pump();

    expect(client.calls, isEmpty);
    expect(find.byKey(ReportSheet.sheetKey), findsOneWidget);
    expect(find.text(l10n.reportCommentRequired), findsOneWidget);
    expect(find.byType(SnackBar), findsNothing);
  });

  testWidgets('completion after cancelling the sheet shows no late notice', (
    tester,
  ) async {
    await _bindViewport(tester, const Size(390, 844));
    final pending = Completer<ModerationApiResult<ReportSubmission>>();
    final client = _FakeModerationClient(pending: pending);
    final focusNode = await _pumpReportPage(tester, client);
    addTearDown(focusNode.dispose);

    await _openAndSelectSpam(tester, focusNode);
    await tester.tap(find.byKey(ReportSheet.submitButtonKey));
    await tester.pump();
    expect(client.calls, hasLength(1));

    await tester.tapAt(const Offset(8, 8));
    await tester.pumpAndSettle();
    expect(find.byKey(ReportSheet.sheetKey), findsNothing);
    pending.complete(
      const ModerationApiOk(ReportSubmission(reportId: 'late-fixture')),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(_activePageKey), findsOneWidget);
    expect(find.byType(SnackBar), findsNothing);
  });
}

Future<void> _bindViewport(WidgetTester tester, Size size) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.binding.setSurfaceSize(size);
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

Future<FocusNode> _pumpReportPage(
  WidgetTester tester,
  _FakeModerationClient client,
) async {
  final focusNode = FocusNode(debugLabel: 'active report page field');
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        voiceModerationClientProvider.overrideWithValue(client),
        authorizationHeaderProvider.overrideWith(
          (ref) => 'Bearer test-session',
        ),
      ],
      child: MaterialApp(
        locale: const Locale('ru'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        theme: _appTheme,
        home: Scaffold(
          appBar: AppBar(title: const Text('Чаты')),
          body: Padding(
            key: _activePageKey,
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const ListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text('Общий чат'),
                  subtitle: Text('Активная страница'),
                ),
                const Expanded(
                  child: Align(
                    alignment: Alignment.topLeft,
                    child: Text('Последнее сообщение в чате'),
                  ),
                ),
                TextField(
                  key: _activePageFieldKey,
                  focusNode: focusNode,
                  decoration: const InputDecoration(labelText: 'Сообщение'),
                ),
                const SizedBox(height: 8),
                FilledButton.tonalIcon(
                  key: _openReportKey,
                  onPressed: () => ReportSheet.show(
                    tester.element(find.byKey(_openReportKey)),
                    target: const ReportMessageTarget(
                      messageId: 'message-42',
                      chatId: 'chat-7',
                    ),
                  ),
                  icon: const Icon(Icons.flag_outlined),
                  label: const Text('Пожаловаться'),
                ),
              ],
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return focusNode;
}

Future<void> _openReport(WidgetTester tester, FocusNode focusNode) async {
  await tester.tap(find.byKey(_activePageFieldKey));
  await tester.pump();
  expect(focusNode.hasFocus, isTrue);
  await tester.tap(find.byKey(_openReportKey));
  await tester.pumpAndSettle();
  expect(find.byKey(ReportSheet.sheetKey), findsOneWidget);
}

Future<void> _openAndSelectSpam(
  WidgetTester tester,
  FocusNode focusNode,
) async {
  await _openReport(tester, focusNode);
  final l10n = AppLocalizations.of(tester.element(find.byKey(_openReportKey)))!;
  await tester.tap(
    find.widgetWithText(RadioListTile<String>, l10n.reportCategorySpam),
  );
  await tester.pump();
  await tester.ensureVisible(find.byKey(ReportSheet.submitButtonKey));
}

void _expectSuccessfulSubmission(
  WidgetTester tester,
  _FakeModerationClient client,
  FocusNode focusNode,
) {
  final l10n = AppLocalizations.of(tester.element(find.byKey(_activePageKey)))!;
  expect(find.byKey(ReportSheet.sheetKey), findsNothing);
  expect(find.byKey(ReportSheet.acceptedKey), findsNothing);
  expect(find.byKey(_activePageKey), findsOneWidget);
  expect(find.byType(SnackBar), findsOneWidget);
  expect(find.text(l10n.reportAcceptedTitle), findsOneWidget);
  expect(find.text(l10n.reportAcceptedMessage), findsOneWidget);
  expect(find.text('report-fixture'), findsNothing);
  expect(tester.widget<SnackBar>(find.byType(SnackBar)).showCloseIcon, isTrue);
  expect(focusNode.hasFocus, isTrue);
  expect(client.calls, hasLength(1));
  expect(client.calls.single.authorization, 'Bearer test-session');
  expect(client.calls.single.targetType, 'message');
  expect(client.calls.single.targetId, 'message-42');
  expect(client.calls.single.category, ReportCategories.spam);
  expect(client.calls.single.description, isNull);
  expect(client.calls.single.evidence, {
    'chat_id': 'chat-7',
    'message_id': 'message-42',
  });
}

class _ReportCall {
  const _ReportCall({
    required this.authorization,
    required this.targetType,
    required this.targetId,
    required this.category,
    required this.description,
    required this.evidence,
  });

  final String authorization;
  final String targetType;
  final String targetId;
  final String category;
  final String? description;
  final Map<String, Object?> evidence;
}

class _FakeModerationClient implements VoiceModerationClient {
  _FakeModerationClient({this.result, this.pending});

  final ModerationApiResult<ReportSubmission>? result;
  final Completer<ModerationApiResult<ReportSubmission>>? pending;
  final List<_ReportCall> calls = [];

  @override
  Future<ModerationApiResult<ReportSubmission>> createReport({
    required String authorization,
    required String targetType,
    required String targetId,
    required String category,
    String? description,
    Map<String, Object?> evidence = const {},
  }) {
    calls.add(
      _ReportCall(
        authorization: authorization,
        targetType: targetType,
        targetId: targetId,
        category: category,
        description: description,
        evidence: Map.unmodifiable(evidence),
      ),
    );
    final deferred = pending;
    if (deferred != null) return deferred.future;
    return Future.value(result!);
  }

  @override
  Future<ModerationApiResult<AppealSubmission>> submitAppeal({
    required String authorization,
    required String sanctionId,
    required String reason,
  }) => throw UnimplementedError();

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}
