import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/presigned_upload.dart';

void main() {
  test('rewritePresignedUrlForHost maps compose MinIO hosts to localhost', () {
    expect(
      rewritePresignedUrlForHost(
        Uri.parse('http://host.docker.internal:9000/bucket/key'),
      ).toString(),
      'http://127.0.0.1:9000/bucket/key',
    );
    expect(
      rewritePresignedUrlForHost(
        Uri.parse('http://minio:9000/bucket/key'),
      ).toString(),
      'http://127.0.0.1:9000/bucket/key',
    );
    expect(
      rewritePresignedUrlForHost(
        Uri.parse('https://cdn.example/avatars/x.png'),
      ).toString(),
      'https://cdn.example/avatars/x.png',
    );
  });
}
