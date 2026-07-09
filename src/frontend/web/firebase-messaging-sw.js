// Firebase Cloud Messaging service worker (notifications (docs/features/notifications.md)).
// Replace firebaseConfig with project values from Firebase console / dart-define.
importScripts('https://www.gstatic.com/firebasejs/10.14.1/firebase-app-compat.js');
importScripts('https://www.gstatic.com/firebasejs/10.14.1/firebase-messaging-compat.js');

firebase.initializeApp({
  apiKey: self.FIREBASE_API_KEY || 'dev',
  authDomain: self.FIREBASE_AUTH_DOMAIN || 'dev',
  projectId: self.FIREBASE_PROJECT_ID || 'dev',
  messagingSenderId: self.FIREBASE_MESSAGING_SENDER_ID || 'dev',
  appId: self.FIREBASE_APP_ID || 'dev',
});

const messaging = firebase.messaging();
messaging.onBackgroundMessage((payload) => {
  // OS shows notification; app handles tap via getInitialMessage/onMessageOpenedApp.
  return payload;
});
