import { useEffect, memo } from 'react';
import styled from 'styled-components';
import { motion, useSpring, useTransform } from 'framer-motion';

const Style = {
  CounterContainer: styled.div`
    display: inline-flex;
    font-family: ${p => p.theme.fonts.typewriter || 'monospace'};
    font-weight: bold;
    overflow: hidden;
    height: 1.2em;
    line-height: 1.2em;
    position: relative;
  `,
  DigitWrapper: styled.div`
    position: relative;
    width: 0.6em;
    height: 100%;
  `,
  DigitColumn: styled(motion.div)`
    display: flex;
    flex-direction: column;
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
  `,
  Digit: styled.span`
    display: flex;
    align-items: center;
    justify-content: center;
    height: 1.2em;
    font-variant-numeric: tabular-nums;
  `,
};

interface RollingDigitProps {
  value: number;
}

const RollingDigit = memo(({ value }: RollingDigitProps) => {
  const spring = useSpring(value, {
    damping: 20,
    stiffness: 100,
    mass: 0.8,
  });

  // Automatically animates when 'value' prop changes
  useEffect(() => {
    spring.set(value);
  }, [value, spring]);

  const y = useTransform(spring, v => `${-v * 10}%`);

  return (
    <Style.DigitWrapper>
      <Style.DigitColumn style={{ y }}>
        {[0, 1, 2, 3, 4, 5, 6, 7, 8, 9].map(digit => (
          <Style.Digit key={digit}>{digit}</Style.Digit>
        ))}
      </Style.DigitColumn>
    </Style.DigitWrapper>
  );
});

interface RollingCounterProps {
  value: number;
  decimals?: number;
  prefix?: string;
  suffix?: string;
}

export const RollingCounter = memo(
  ({ value, decimals = 0, prefix = '', suffix = '' }: RollingCounterProps) => {
    const safeValue = typeof value === 'number' && !isNaN(value) ? value : 0;
    const formatted = safeValue.toFixed(decimals);

    return (
      <Style.CounterContainer>
        {prefix && <span>{prefix}</span>}
        {Array.from(formatted).map((char, i) => {
          const num = parseInt(char, 10);
          if (isNaN(num)) {
            return <span key={i}>{char}</span>;
          }
          // FIX: Use index as key to persist component and enable animation
          return <RollingDigit key={i} value={num} />;
        })}
        {suffix && <span>{suffix}</span>}
      </Style.CounterContainer>
    );
  }
);

export default RollingCounter;
