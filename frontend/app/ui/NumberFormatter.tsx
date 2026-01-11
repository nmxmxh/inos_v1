import React from 'react';
import styled from 'styled-components';
import { motion } from 'framer-motion';
import RollingCounter from './RollingCounter';

const Style = {
  Container: styled.div`
    display: inline-flex;
    align-items: baseline;
    font-variant-numeric: tabular-nums;
  `,
  Value: styled(motion.span)`
    font-weight: 700;
  `,
  Suffix: styled.span`
    font-size: 0.8em;
    opacity: 0.7;
    margin-left: 2px;
  `,
};

interface NumberFormatterProps {
  value: number | bigint;
  decimals?: number;
  suffix?: string;
}

export const NumberFormatter: React.FC<NumberFormatterProps> = ({
  value,
  decimals = 1,
  suffix = '',
}) => {
  const numCheck = typeof value === 'bigint' ? Number(value) : value;

  let displayValue = numCheck;
  let unit = '';

  if (numCheck >= 1_000_000_000_000) {
    displayValue = numCheck / 1_000_000_000_000;
    unit = 'T';
  } else if (numCheck >= 1_000_000_000) {
    displayValue = numCheck / 1_000_000_000;
    unit = 'G';
  } else if (numCheck >= 1_000_000) {
    displayValue = numCheck / 1_000_000;
    unit = 'M';
  } else if (numCheck >= 1_000) {
    displayValue = numCheck / 1_000;
    unit = 'K';
  }

  // Format with fixed decimals if abbreviated, or locale string if small
  // We pass the raw displayValue to RollingCounter which handles rounding and animation

  return (
    <Style.Container>
      <RollingCounter value={displayValue} decimals={unit ? decimals : 0} />
      {(unit || suffix) && (
        <Style.Suffix>
          {unit}
          {suffix}
        </Style.Suffix>
      )}
    </Style.Container>
  );
};

export default NumberFormatter;
