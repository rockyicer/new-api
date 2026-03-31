/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import { API, getLogo, getSystemName, showError } from '../../helpers';
import { marked } from 'marked';
import { Button, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { IconPlay } from '@douyinfe/semi-icons';
import { Link } from 'react-router-dom';

const About = () => {
  const { t } = useTranslation();
  const [about, setAbout] = useState('');
  const [aboutLoaded, setAboutLoaded] = useState(false);
  const systemName = getSystemName();
  const logo = getLogo();

  const displayAbout = async () => {
    setAbout(localStorage.getItem('about') || '');
    const res = await API.get('/api/about');
    const { success, message, data } = res.data;
    if (success) {
      let aboutContent = data;
      if (!data.startsWith('https://')) {
        aboutContent = marked.parse(data);
      }
      setAbout(aboutContent);
      localStorage.setItem('about', aboutContent);
    } else {
      showError(message);
      setAbout(t('加载关于内容失败...'));
    }
    setAboutLoaded(true);
  };

  useEffect(() => {
    displayAbout().then();
  }, []);

  return (
    <div className='mt-[60px] px-2'>
      {aboutLoaded && about === '' ? (
        <div className='relative min-h-[calc(100vh-60px)] overflow-hidden'>
          <div className='blur-ball blur-ball-indigo' />
          <div className='blur-ball blur-ball-teal' />

          <div className='mx-auto max-w-6xl px-4 py-10 md:py-14 lg:py-20'>
            <div
              className='rounded-[32px] border p-8 md:p-10 lg:p-12 shadow-sm'
              style={{
                background: 'var(--semi-color-bg-0)',
                borderColor: 'var(--semi-color-border)',
              }}
            >
              <div className='flex flex-col gap-8 lg:flex-row lg:items-start lg:justify-between'>
                <div className='max-w-3xl'>
                  <div className='flex items-center gap-8 md:gap-10'>
                    <div
                      className='flex h-28 w-28 shrink-0 items-center justify-center rounded-[36px] border md:h-36 md:w-36'
                      style={{
                        background: 'var(--semi-color-bg-1)',
                        borderColor: 'var(--semi-color-border)',
                      }}
                    >
                      <img
                        src={logo}
                        alt={systemName}
                        className='h-16 w-16 object-contain md:h-[84px] md:w-[84px]'
                      />
                    </div>
                    <div>
                      <div className='text-xs font-semibold uppercase tracking-[0.32em] text-semi-color-text-2'>
                        {systemName}
                      </div>
                      <h1 className='mt-3 text-4xl font-bold leading-[0.96] text-semi-color-text-0 md:text-5xl lg:text-6xl'>
                        <span className='shine-text'>
                          {t('让大家都用得起 API')}
                        </span>
                      </h1>
                    </div>
                  </div>

                  <p className='mt-6 text-base leading-8 text-semi-color-text-1 md:text-lg'>
                    {t(
                      'JUSTAPI 成立于 2026，专注于把多模型接入、统一网关、稳定调用与透明计费整合成一套更易用的产品体验。',
                    )}
                  </p>
                  <p className='mt-4 text-base leading-8 text-semi-color-text-1 md:text-lg'>
                    {t(
                      '我们相信，API 不应该只是少数团队的基础设施，而应该成为每一位开发者、创业团队和企业都接得上、跑得稳、负担得起的能力底座。',
                    )}
                  </p>

                  <div className='mt-8 flex flex-wrap gap-3'>
                    <Link to='/console'>
                      <Button
                        theme='solid'
                        type='primary'
                        size='large'
                        className='!rounded-3xl px-8'
                        icon={<IconPlay />}
                      >
                        {t('获取密钥')}
                      </Button>
                    </Link>
                  </div>
                </div>

                <div className='grid w-full max-w-xl gap-4 md:grid-cols-3 lg:max-w-md lg:grid-cols-1'>
                  {[
                    {
                      title: t('公司介绍'),
                      value: systemName,
                      description: t(
                        'JUSTAPI 是一家专注于 API 服务体验与模型接入效率的技术团队。',
                      ),
                    },
                    {
                      title: t('成立时间'),
                      value: '2026',
                      description: t('从产品第一天开始，就以长期可交付为目标。'),
                    },
                    {
                      title: t('我们的目标'),
                      value: t('让每一位开发者、团队与企业都用得起 API。'),
                      description: t(
                        '把复杂留在系统内部，把简单、高效和稳定交给每一位用户。',
                      ),
                    },
                  ].map((item) => (
                    <div
                      key={item.title}
                      className='rounded-[24px] border p-5'
                      style={{
                        background: 'var(--semi-color-fill-0)',
                        borderColor: 'var(--semi-color-border)',
                      }}
                    >
                      <div className='text-sm font-medium text-semi-color-text-2'>
                        {item.title}
                      </div>
                      <div className='mt-3 text-xl font-semibold leading-8 text-semi-color-text-0'>
                        {item.value}
                      </div>
                      <div className='mt-3 text-sm leading-6 text-semi-color-text-1'>
                        {item.description}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            <div className='mt-6 grid gap-4 lg:grid-cols-[1.2fr,0.8fr]'>
              <div
                className='rounded-[28px] border p-8 md:p-10'
                style={{
                  background: 'var(--semi-color-bg-0)',
                  borderColor: 'var(--semi-color-border)',
                }}
              >
                <Typography.Title
                  heading={3}
                  className='!mb-4 !text-semi-color-text-0'
                >
                  {t('为什么是 JUSTAPI')}
                </Typography.Title>
                <div className='space-y-4 text-base leading-8 text-semi-color-text-1'>
                  <p>
                    {t(
                      '从模型接入到版本迭代，从价格透明到交付稳定，JUSTAPI 希望把复杂留在系统内部，把简单交给每一位用户。',
                    )}
                  </p>
                  <p>
                    {t(
                      '无论你是个人开发者、小型团队还是正在扩张的企业，我们都希望你能用更低的门槛接入能力，用更可控的成本完成产品落地。',
                    )}
                  </p>
                </div>
              </div>

              <div className='grid gap-4'>
                {[
                  {
                    title: t('统一接入'),
                    description: t(
                      '统一管理多家模型供应商，减少切换与维护成本。',
                    ),
                  },
                  {
                    title: t('透明成本'),
                    description: t(
                      '提供更清晰的价格结构，让预算与调用规模更容易规划。',
                    ),
                  },
                  {
                    title: t('稳定交付'),
                    description: t(
                      '持续优化网关能力，让开发、测试与生产发布更顺滑。',
                    ),
                  },
                ].map((item) => (
                  <div
                    key={item.title}
                    className='rounded-[24px] border p-6'
                    style={{
                      background: 'var(--semi-color-bg-0)',
                      borderColor: 'var(--semi-color-border)',
                    }}
                  >
                    <div className='text-lg font-semibold text-semi-color-text-0'>
                      {item.title}
                    </div>
                    <div className='mt-2 text-sm leading-7 text-semi-color-text-1'>
                      {item.description}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      ) : (
        <>
          {about.startsWith('https://') ? (
            <iframe
              src={about}
              style={{ width: '100%', height: '100vh', border: 'none' }}
            />
          ) : (
            <div
              style={{ fontSize: 'larger' }}
              dangerouslySetInnerHTML={{ __html: about }}
            ></div>
          )}
        </>
      )}
    </div>
  );
};

export default About;
