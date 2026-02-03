export interface PlatformInfo {
  userAgent: string;
  maxTouchPoints: number;
  isIOS: boolean;
  isSafari: boolean;
  isWebKit: boolean;
  isMobile: boolean;
}

export function getPlatformInfo(): PlatformInfo {
  const nav = typeof navigator !== 'undefined' ? navigator : undefined;
  const userAgent = nav?.userAgent || '';
  const maxTouchPoints = nav?.maxTouchPoints || 0;

  const isIOS =
    /iPad|iPhone|iPod/.test(userAgent) ||
    (userAgent.includes('Mac') && maxTouchPoints > 1);

  const isSafari =
    /safari/i.test(userAgent) &&
    !/chrome|android|crios|fxios|edg\//i.test(userAgent);

  const isWebKit =
    /applewebkit/i.test(userAgent) &&
    !/chrome|crios|edg\//i.test(userAgent);

  const isMobile = /mobile|iphone|ipad|ipod|android/i.test(userAgent);

  return {
    userAgent,
    maxTouchPoints,
    isIOS,
    isSafari,
    isWebKit,
    isMobile,
  };
}
