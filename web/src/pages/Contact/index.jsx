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

import React from 'react';
import { Button, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { getLogo, getSystemName } from '../../helpers';
import { IconPlay } from '@douyinfe/semi-icons';
import { Link } from 'react-router-dom';

const Contact = () => {
  const { t } = useTranslation();
  const systemName = getSystemName();
  const logo = getLogo();

  const supportGroups = [
    {
      title: t('QQ 帮助群'),
      status: t('当前已开放'),
      description: t('用于反馈部署、调用异常、计费疑问与使用建议。'),
      image: '/qq-team.jpg',
      imageAlt: t('QQ 帮助群二维码'),
    },
    {
      title: t('Telegram 帮助群'),
      status: t('二维码预留'),
      description: t('海外用户支持与问题反馈通道正在准备中。'),
      placeholder: t('预留 Telegram 二维码位置'),
    },
    {
      title: t('WhatsApp 帮助群'),
      status: t('二维码预留'),
      description: t('移动端沟通与问题反馈通道正在准备中。'),
      placeholder: t('预留 WhatsApp 二维码位置'),
    },
  ];

  const highlights = [
    {
      title: t('问题反馈'),
      description: t('遇到异常、接口报错或体验问题时，可以在群内直接反馈。'),
    },
    {
      title: t('部署支持'),
      description: t('部署、配置、鉴权或渠道接入问题可以在这里交流。'),
    },
    {
      title: t('使用建议'),
      description: t('我们会持续收集需求与建议，用于后续迭代优化。'),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
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
                <div className='flex items-center gap-4'>
                  <div
                    className='flex h-16 w-16 items-center justify-center rounded-2xl border'
                    style={{
                      background: 'var(--semi-color-bg-1)',
                      borderColor: 'var(--semi-color-border)',
                    }}
                  >
                    <img
                      src={logo}
                      alt={systemName}
                      className='h-10 w-10 object-contain'
                    />
                  </div>
                  <div>
                    <div className='text-xs font-semibold uppercase tracking-[0.32em] text-semi-color-text-2'>
                      {systemName}
                    </div>
                    <h1 className='mt-2 text-4xl font-bold leading-tight text-semi-color-text-0 md:text-5xl lg:text-6xl'>
                      <span className='shine-text'>{t('反馈问题与帮助群')}</span>
                    </h1>
                  </div>
                </div>

                <p className='mt-6 text-base leading-8 text-semi-color-text-1 md:text-lg'>
                  {t(
                    '如果你在部署、调用、计费或日常使用过程中遇到问题，欢迎加入帮助群反馈，我们会持续查看并跟进。',
                  )}
                </p>
                <p className='mt-4 text-base leading-8 text-semi-color-text-1 md:text-lg'>
                  {t(
                    '这些帮助群主要用于反馈问题、同步处理进展与收集使用建议。',
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
                      {t('进入控制台')}
                    </Button>
                  </Link>
                </div>
              </div>

              <div className='grid w-full max-w-xl gap-4 md:grid-cols-3 lg:max-w-md lg:grid-cols-1'>
                {highlights.map((item) => (
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
                    <div className='mt-3 text-sm leading-7 text-semi-color-text-1'>
                      {item.description}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className='mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
            {supportGroups.map((group) => (
              <div
                key={group.title}
                className='rounded-[28px] border p-6 md:p-7'
                style={{
                  background: 'var(--semi-color-bg-0)',
                  borderColor: 'var(--semi-color-border)',
                }}
              >
                <div className='flex items-start justify-between gap-4'>
                  <div>
                    <div className='text-xl font-semibold text-semi-color-text-0'>
                      {group.title}
                    </div>
                    <div className='mt-3 text-sm leading-7 text-semi-color-text-1'>
                      {group.description}
                    </div>
                  </div>
                  <span
                    className='rounded-full px-3 py-1 text-xs font-semibold'
                    style={{
                      background: 'var(--semi-color-fill-0)',
                      color: 'var(--semi-color-primary)',
                    }}
                  >
                    {group.status}
                  </span>
                </div>

                <div className='mt-6'>
                  {group.image ? (
                    <div
                      className='overflow-hidden rounded-[24px] border p-4'
                      style={{
                        background: 'var(--semi-color-bg-1)',
                        borderColor: 'var(--semi-color-border)',
                      }}
                    >
                      <img
                        src={group.image}
                        alt={group.imageAlt}
                        className='aspect-square w-full rounded-[18px] object-cover'
                      />
                    </div>
                  ) : (
                    <div
                      className='flex aspect-square items-center justify-center rounded-[24px] border border-dashed px-6 text-center text-sm leading-7 text-semi-color-text-2'
                      style={{
                        background: 'var(--semi-color-bg-1)',
                        borderColor: 'var(--semi-color-border)',
                      }}
                    >
                      {group.placeholder}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>

          <div
            className='mt-6 rounded-[28px] border p-6 md:p-8'
            style={{
              background: 'var(--semi-color-bg-0)',
              borderColor: 'var(--semi-color-border)',
            }}
          >
            <Typography.Title
              heading={4}
              className='!mb-3 !text-semi-color-text-0'
            >
              {t('反馈部署、调用、计费与使用问题')}
            </Typography.Title>
            <div className='text-sm leading-7 text-semi-color-text-1 md:text-base'>
              {t(
                '如果你发现接口异常、模型调用不稳定、计费疑问或产品建议，欢迎通过这些帮助群集中反馈，我们会按优先级持续跟进。',
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Contact;
